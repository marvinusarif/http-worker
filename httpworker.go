package httpworker

import (
	"errors"
	"fmt"
	"net/http"
	"sync"
	"time"
)

var ErrHTTPWorkerUnavailable = errors.New("HTTPWorkPool: timeout - worker unavailable")

type Pool interface {
	Do(httpRequest *http.Request) (*http.Response, error)
	Terminate()
}

type PoolImpl struct {
	workResponsePool *sync.Pool
	MaxConn          int
	Requests         chan *HTTPWorkRequest
	Client           *http.Client
	quits            []chan struct{}
}

type HTTPWorkRequest struct {
	mu        *sync.Mutex
	Request   *http.Request
	Response  *HTTPWorkResponse
	response  chan *HTTPWorkResponse
	cancelled bool
	replied   bool
}

type HTTPWorkResponse struct {
	Response *http.Response
	Error    error
}

func NewInitPool(maxConn, maxIdleConnPerHost int, timeout time.Duration) Pool {
	if maxIdleConnPerHost < 1 {
		maxIdleConnPerHost = maxConn
	}

	defaultRoundTripper := http.DefaultTransport
	defaultTransport, ok := defaultRoundTripper.(*http.Transport)
	if !ok {
		panic(fmt.Sprintf("default Roundtripper is not *http.Transport"))
	}
	defaultTransport.MaxIdleConns = maxConn
	defaultTransport.MaxConnsPerHost = maxIdleConnPerHost
	defaultTransport.MaxIdleConnsPerHost = maxIdleConnPerHost
	pool := &PoolImpl{
		workResponsePool: &sync.Pool{
			New: func() interface{} {
				return new(HTTPWorkResponse)
			},
		},
		MaxConn:  maxConn,
		Client:   &http.Client{Transport: defaultTransport},
		Requests: make(chan *HTTPWorkRequest, maxConn),
		quits:    make([]chan struct{}, 0),
	}
	pool.NewWorkerPool()
	return pool
}

func (p *PoolImpl) NewWorkerPool() {
	for i := 0; i < p.MaxConn; i++ {
		quitChan := make(chan struct{})
		go p.NewHTTPWorker(p.Requests, quitChan)
		p.quits = append(p.quits, quitChan)
	}
}

func (p *PoolImpl) NewHTTPWorker(httpWorkRequests <-chan *HTTPWorkRequest, quitChan <-chan struct{}) {
	for {
		select {
		case <-quitChan:
			return
		case httpWorkRequest := <-httpWorkRequests:
			p.do(httpWorkRequest)
		}
	}
}

func (p *PoolImpl) do(httpWorkRequest *HTTPWorkRequest) {
	resp, err := p.Client.Do(httpWorkRequest.Request)
	if err != nil {
		p.SendReply(httpWorkRequest, &HTTPWorkResponse{resp, err})
		return
	}
	p.SendReply(httpWorkRequest, &HTTPWorkResponse{resp, err})
}

func (p *PoolImpl) SendReply(httpWorkRequest *HTTPWorkRequest, httpWorkResponse *HTTPWorkResponse) {
	httpWorkRequest.mu.Lock()
	cancelled := httpWorkRequest.cancelled
	httpWorkRequest.replied = true
	defer httpWorkRequest.mu.Unlock()
	if !cancelled {
		go func() {
			httpWorkRequest.response <- httpWorkResponse
		}()
	}
}

func (p *PoolImpl) Do(httpWorkRequest *http.Request) (*http.Response, error) {

	httpWorkResponse := p.workResponsePool.Get().(*HTTPWorkResponse)
	defer p.workResponsePool.Put(httpWorkResponse)

	HTTPWorkRequest := &HTTPWorkRequest{
		mu:       &sync.Mutex{},
		Request:  httpWorkRequest,
		Response: httpWorkResponse,
		response: make(chan *HTTPWorkResponse),
	}

	go p.SendHTTPRequestToWorker(HTTPWorkRequest)

	select {
	case <-httpWorkRequest.Context().Done():
		HTTPWorkRequest.mu.Lock()
		replied := HTTPWorkRequest.replied
		HTTPWorkRequest.cancelled = true
		if !replied {
			HTTPWorkRequest.Response.Error = ErrHTTPWorkerUnavailable
		}
		HTTPWorkRequest.mu.Unlock()
	case HTTPWorkRequest.Response = <-HTTPWorkRequest.response:
	}
	return HTTPWorkRequest.Response.Response, HTTPWorkRequest.Response.Error
}

func (p *PoolImpl) SendHTTPRequestToWorker(httpWorkRequest *HTTPWorkRequest) {
	p.Requests <- httpWorkRequest
}

func (p *PoolImpl) Terminate() {
	for _, q := range p.quits {
		q <- struct{}{}
	}
}
