package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"sync"
	"sync/atomic"
	"time"

	"github.com/google/gops/agent"
	httpworker "github.com/tokopedia/http-worker"
)

func main() {
	if err := agent.Listen(agent.Options{}); err != nil {
		log.Fatal(err)
	}

	var success, failed uint64
	httpWorker := httpworker.NewInitPool(1000, 1000, 10*time.Second)
	requests := 10000
	wg := &sync.WaitGroup{}
	wg.Add(requests)
	now := time.Now()
	fmt.Println("Requests start!")
	for i := 1; i <= requests; i++ {
		go func(i int) {
			defer wg.Done()
			ctx, cancel := context.WithTimeout(context.Background(), time.Duration(i)*time.Second)
			defer cancel()

			req, err := http.NewRequest("GET", "https://www.google.com", nil)
			if err != nil {
				panic(err)
			}
			req = req.WithContext(ctx)
			resp, err := httpWorker.Do(req)
			if err != nil {
				atomic.AddUint64(&failed, 1)
				fmt.Println("request (fail) #", i, "error:", err)
			} else {
				atomic.AddUint64(&success, 1)
				fmt.Println("request (succ) #", i, "with status code:", resp.StatusCode)
			}
		}(i)
	}
	wg.Wait()
	fmt.Println("Requests end in", time.Since(now), "failed:", failed, "success:", success)
}
