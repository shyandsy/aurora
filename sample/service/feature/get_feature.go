package feature

import (
	"fmt"

	"github.com/shyandsy/aurora/bizerr"
	"github.com/shyandsy/aurora/contracts"
	"github.com/shyandsy/aurora/sample/model/dto"
)

func (s *featureService) GetFeature(ctx *contracts.RequestContext, id int64) (*dto.Feature, bizerr.BizError) {
	feature, err := s.DL.GetByID(ctx.Context, id)
	if err != nil {
		return nil, bizerr.ErrInternalServerError(fmt.Errorf("%s: %w", ctx.T("error.internal_server"), err))
	}
	if feature == nil {
		msg := ctx.T("feature.not_found")
		return nil, bizerr.NewValidationError(msg, nil)
	}

	return feature.ToDto(), nil
}
