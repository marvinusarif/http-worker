package httpworker_test

import (
	"context"
	"io/ioutil"
	"net/http"
	"testing"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/ghttp"
	httpworker "github.com/tokopedia/http-worker"
)

var _ = Describe("HTTPWorker Test", func() {
	var (
		server     *ghttp.Server
		httpWorker httpworker.Pool
		req        *http.Request
	)

	BeforeEach(func() {
		httpWorker = httpworker.NewInitPool(1000, 1000, 10*time.Second)
		server = ghttp.NewServer()
	})

	Describe("HTTP Client", func() {
		var (
			result     []byte
			statusCode int
		)
		Context("make http call without context", func() {

			BeforeEach(func() {
				req, _ = http.NewRequest("GET", server.URL(), nil)
				req = req.WithContext(context.Background())

				statusCode = http.StatusOK
				result = []byte("this is response")
				server.AppendHandlers(
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("GET", "/"),
						ghttp.RespondWith(statusCode, result),
					),
				)
			})

			It("should make request to server", func() {
				_, err := httpWorker.Do(req)
				Ω(err).ShouldNot(HaveOccurred())
				Ω(server.ReceivedRequests()).Should(HaveLen(1))
			})

			It("should return response", func() {
				resp, err := httpWorker.Do(req)
				Ω(err).ShouldNot(HaveOccurred())
				body, _ := ioutil.ReadAll(resp.Body)
				Ω(body).Should(Equal(result))
			})
		})

		Context("create http call with timeout context", func() {

			BeforeEach(func() {
				ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
				defer cancel()
				req, _ = http.NewRequest("GET", server.URL(), nil)
				req = req.WithContext(ctx)

				statusCode = http.StatusOK
				result = []byte("this is response")
				server.AppendHandlers(
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("GET", "/"),
						ghttp.RespondWith(statusCode, result),
					),
				)
			})

			It("should return error and nil response", func() {
				resp, err := httpWorker.Do(req)
				Ω(err).Should(HaveOccurred())
				Ω(resp).Should(BeNil())
			})

		})
	})

	AfterEach(func() {
		//shut down the server between tests
		server.Close()
	})
})

func TestHttpWorker(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "HttpWorker Suite")
}
