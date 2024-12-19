package validate

import (
	"github.com/pkg/errors"

	"github.com/superpowerdotcom/events/build/proto/go/affiliate"
)

func Affiliate(affiliate *affiliate.Affiliate) error {
	if affiliate == nil {
		return errors.New("affiliate cannot be nil")
	}

	if affiliate.RewardfulId == "" {
		return errors.New("rewardful ID cannot be empty")
	}

	if err := User(affiliate.User); err != nil {
		return errors.Wrap(err, "failed to validate user")
	}

	return nil
}
