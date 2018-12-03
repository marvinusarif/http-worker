package httpworker

/*
a HTTP version of this redis pooler:
https://gist.github.com/codemartial/625f1cbfe030f39451bb5785ccbf335e

06 May 2017:
Experiment with WorkerPool for doing HTTPRequest
Implements Bulkhead Pattern: https://docs.microsoft.com/en-us/azure/architecture/patterns/bulkhead
*/

import (
	"errors"
	"net/http"
	"time"
)

// HTTPWork contains the input of work which *http.Request and response channel.
// Once submited to worker HTTPWorker, HTTPWorker will return the response through res channel.
type HTTPWork struct {
	req *http.Request
	res chan HTTPWorkResponse
}

// HTTPWorkResponse is response of a work
type HTTPWorkResponse struct {
	Response *http.Response
	Error    error
}

// ErrHTTPWorkerUnavailable returned when qtimeout reached after waiting for available HTTPWorker.
var ErrHTTPWorkerUnavailable = errors.New("HTTPWorkPool: timeout - worker unavailiable")

// Pool is a pooler for HTTPWorkers. Work HTTPWork will be submited to the worker pool, then
// Pool will asign the work to available Worker
type Pool struct {
	queue      chan HTTPWork
	qtimeout   time.Duration
	w          []*HTTPWorker
	httpclient *http.Client
}

// PoolOption is params for initializing HTTPWorkerPool
// N number of worker HTTPWorker to be created
// MaxBacklog is estimate of how many work can be queued
type PoolOption struct {
	N            int
	MaxBacklog   int
	HTTPClient   *http.Client
	QueueTimeout time.Duration
}

// NewHTTPWorkerPool returns new HTTPWorkerPool
func NewHTTPWorkerPool(opt *PoolOption) (*Pool, error) {

	hc := &http.Client{}
	if opt.HTTPClient != nil {
		hc = opt.HTTPClient
	}

	// buffered channel of HTTPWork
	q := make(chan HTTPWork, opt.MaxBacklog*opt.N)
	// HTTPWorkers
	workers := make([]*HTTPWorker, 0, opt.N)
	for i := 0; i < opt.N; i++ {
		quitchan := make(chan struct{}, 1)
		w := HTTPWorker{httpclient: hc, queue: q, quit: quitchan}
		go w.StartWorking()
		workers = append(workers, &w)
	}

	timeout := 100 * time.Millisecond
	if opt.QueueTimeout != 0 {
		timeout = opt.QueueTimeout
	}

	return &Pool{
		queue:      q,
		qtimeout:   timeout,
		w:          workers,
		httpclient: hc,
	}, nil
}

// Do of HTTPWorkerPool
func (wp *Pool) Do(req *http.Request) HTTPWorkResponse {
	// return channel
	rchan := make(chan HTTPWorkResponse, 1)
	if err := wp.submitWork(HTTPWork{req: req, res: rchan}); err != nil {
		return HTTPWorkResponse{nil, err}
	}

	return <-rchan
}

func (wp *Pool) submitWork(work HTTPWork) error {
	select {
	case wp.queue <- work:
		return nil
	case <-time.After(wp.qtimeout):
		return ErrHTTPWorkerUnavailable
	}
}

func (wp *Pool) ShutDown() {
	for i, _ := range wp.w {
		wp.w[i].quit <- struct{}{}
	}
}

// HTTPWorker will process all HTTPWork given by the queue with httpclient
type HTTPWorker struct {
	httpclient *http.Client
	queue      chan HTTPWork
	quit       chan struct{}
}

// StartWorking will instruct HTTPWorker to either quit when w.quit is signalled, or to doWork if there is work available
func (w HTTPWorker) StartWorking() {
	defer func() {
		r := recover()

		if r != nil {
			w.StartWorking()
		}
	}()

	for {
		select {
		case <-w.quit:
			return
		case work := <-w.queue:
			w.doWork(work)
		}
	}
}

func (w *HTTPWorker) doWork(work HTTPWork) {
	r, e := w.httpclient.Do(work.req)
	work.res <- HTTPWorkResponse{Response: r, Error: e}
}
