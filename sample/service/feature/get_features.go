package feature

import (
	"fmt"

	"github.com/shyandsy/aurora/bizerr"
	"github.com/shyandsy/aurora/contracts"
	"github.com/shyandsy/aurora/sample/model/dto"
)

func (s *featureService) GetFeatures(ctx *contracts.RequestContext) ([]dto.Feature, bizerr.BizError) {
	features, err := s.DL.GetAll(ctx.Context)
	if err != nil {
		return nil, bizerr.ErrInternalServerError(fmt.Errorf("%s: %w", ctx.T("error.internal_server"), err))
	}

	items := make([]dto.Feature, 0, len(features))
	for _, feature := range features {
		items = append(items, *feature.ToDto())
	}

	return items, nil
}
