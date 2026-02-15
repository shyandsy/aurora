package feature

import (
	"strconv"

	"github.com/shyandsy/aurora/bizerr"
	"github.com/shyandsy/aurora/contracts"
	serviceFeature "github.com/shyandsy/aurora/sample/full_showcase/service/feature"
)

// GetFeature gets feature by ID.
func GetFeature(c *contracts.RequestContext) (interface{}, bizerr.BizError) {
	idStr := c.Param("id")
	if idStr == "" {
		msg := c.T("error.bad_request")
		return nil, bizerr.NewValidationError(msg, nil)
	}

	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil || id <= 0 {
		msg := c.T("error.bad_request")
		return nil, bizerr.NewValidationError(msg, nil)
	}

	var featureService serviceFeature.FeatureService
	if err := c.App.Find(&featureService); err != nil {
		return nil, bizerr.ErrInternalServerError(err)
	}

	feature, bizErr := featureService.GetFeature(c, id)
	if bizErr != nil {
		return nil, bizErr
	}

	return feature, nil
}
