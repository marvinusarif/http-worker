package test

import (
	"context"
	"log"
	"net/http"
	"sync"
	"testing"
	"time"

	httpworker "github.com/tokopedia/http-worker"
	httpworkerV1 "github.com/tokopedia/http-worker/v1"
)

const N int = 500

func BenchmarkHTTPWorker(b *testing.B) {
	httpWorker := httpworker.NewInitPool(N, N, 1*time.Second)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	defer httpWorker.Terminate()

	req, err := http.NewRequest("GET", "http://localhost", nil)
	if err != nil {
		log.Print(err)
	}
	req = req.WithContext(ctx)

	wrappedFunc := func(reqs int) {
		wg := &sync.WaitGroup{}
		wg.Add(reqs)
		for i := 0; i < reqs; i++ {
			go func() (*http.Response, error) {
				defer wg.Done()
				resp, err := httpWorker.Do(req)
				if err != nil {
					return resp, err
				}
				return resp, nil
			}()
		}
		wg.Wait()
	}

	benchmarks := []struct {
		name        string
		wrappedFunc func(reqs int)
		reqs        int
	}{
		{"initiate-ignore this", wrappedFunc, N},
		{"10 req", wrappedFunc, 10},
		{"100 req", wrappedFunc, 100},
		{"1k req", wrappedFunc, 1000},
		{"10k req", wrappedFunc, 10000},
		{"100k req", wrappedFunc, 100000},
	}

	for _, benchmark := range benchmarks {
		b.Run(benchmark.name, func(b *testing.B) {
			b.ReportAllocs()
			b.ResetTimer()
			for n := 0; n < b.N; n++ {
				benchmark.wrappedFunc(benchmark.reqs)
			}
		})
	}
}

func BenchmarkHTTPWorkerV1(b *testing.B) {
	httpWorker, _ := httpworkerV1.NewHTTPWorkerPool(&httpworkerV1.PoolOption{
		N:          N,
		MaxBacklog: 1,
	})
	defer httpWorker.ShutDown()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	req, err := http.NewRequest("GET", "http://localhost", nil)
	if err != nil {
		log.Print(err)
	}
	req = req.WithContext(ctx)

	wrappedFunc := func(reqs int) {
		wg := &sync.WaitGroup{}
		wg.Add(reqs)
		for i := 0; i < reqs; i++ {
			go func() (*http.Response, error) {
				defer wg.Done()
				res := httpWorker.Do(req)
				if res.Error != nil {
					return res.Response, res.Error
				}
				return res.Response, nil
			}()
		}
		wg.Wait()
	}

	benchmarks := []struct {
		name        string
		wrappedFunc func(reqs int)
		reqs        int
	}{
		{"initiate", wrappedFunc, N},
		{"10 req", wrappedFunc, 10},
		{"100 req", wrappedFunc, 100},
		{"1k req", wrappedFunc, 1000},
		{"10k req", wrappedFunc, 10000},
		{"100k req", wrappedFunc, 100000},
	}

	for _, benchmark := range benchmarks {
		b.Run(benchmark.name, func(b *testing.B) {
			b.ReportAllocs()
			b.ResetTimer()
			for n := 0; n < b.N; n++ {
				benchmark.wrappedFunc(benchmark.reqs)
			}
		})
	}
}
