package api

import (
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"go.uber.org/zap"
)

func TestAPISuite(t *testing.T) {
	// Reduce test noise
	zap.IncreaseLevel(zap.FatalLevel)
	RegisterFailHandler(Fail)
	RunSpecs(t, "API Suite")
}
