package api

import (
	. "github.com/onsi/ginkgo"
)

var _ = Describe("API", func() {
	var ()

	BeforeEach(func() {
	})

	Describe("New", func() {
		Context("when instantiating an api", func() {
			It("should have correct attributes", func() {
			})
		})
	})

	//Describe("HealthCheckHandler", func() {
	//	Context("when the request is successful", func() {
	//		It("should return 200", func() {
	//			api.healthCheckHandler(response, request)
	//			Expect(response.Code).To(Equal(200))
	//		})
	//	})
	//})
	//
	//Describe("VersionHandler", func() {
	//	Context("when the request is successful", func() {
	//		It("should return the API version", func() {
	//			api.versionHandler(response, request)
	//			Expect(response.Code).To(Equal(200))
	//			Expect(response.Body).To(ContainSubstring(testVersion))
	//		})
	//	})
	//})
})
