//go:build vectors

package search

import (
	"context"
	"fmt"

	"github.com/blevesearch/bleve/v2"
	"github.com/blevesearch/bleve/v2/mapping"
	"github.com/blevesearch/bleve/v2/search/query"
)

func configureVectorMapping(indexMapping *mapping.IndexMappingImpl, options Options) {
	vectorMapping := mapping.NewVectorFieldMapping()
	vectorMapping.Dims = options.VectorDimensions
	vectorMapping.Similarity = vectorMetricName(options.VectorMetric)
	vectorMapping.VectorIndexOptimizedFor = vectorQualityName(options.VectorQuality)
	indexMapping.DefaultMapping.AddFieldMappingsAt(fieldVector, vectorMapping)
}

func (e *Engine) vectorSearch(ctx context.Context, search Query, limit int) ([]Hit, error) {
	filters, err := e.filterQuery(search.Filters)
	if err != nil {
		return nil, err
	}
	var requestQuery query.Query = bleve.NewMatchNoneQuery()
	if search.Mode == ModeHybrid {
		requestQuery = e.keywordQuery(search.Text)
		if filters != nil {
			requestQuery = bleve.NewConjunctionQuery(requestQuery, filters)
		}
	}
	request := bleve.NewSearchRequestOptions(requestQuery, limit, 0, false)
	request.Fields = hitFields
	knnLimit := max(limit, 100)
	if search.Mode == ModeHybrid {
		request.Score = bleve.ScoreRRF
		request.Params = bleve.NewDefaultParams(0, knnLimit)
	}
	request.AddKNNWithFilter(fieldVector, search.Vector, int64(knnLimit), 1, filters)
	result, err := e.index.SearchInContext(ctx, request)
	if err != nil {
		return nil, fmt.Errorf("Bleve vector search: %w", err)
	}
	var keywordRanks, vectorRanks map[string]int
	if search.Mode == ModeHybrid && search.IncludeRankEvidence {
		keywordRanks, err = e.channelRanks(ctx, e.keywordQuery(search.Text), filters, knnLimit)
		if err != nil {
			return nil, err
		}
		vectorRanks, err = e.vectorRanks(ctx, filters, knnLimit, search.Vector)
		if err != nil {
			return nil, err
		}
	}
	return e.hits(result, search, keywordRanks, vectorRanks)
}

func (e *Engine) vectorRanks(ctx context.Context, filters query.Query, limit int, vector []float32) (map[string]int, error) {
	request := bleve.NewSearchRequestOptions(bleve.NewMatchNoneQuery(), limit, 0, false)
	request.Fields = []string{fieldID}
	request.AddKNNWithFilter(fieldVector, vector, int64(limit), 1, filters)
	result, err := e.index.SearchInContext(ctx, request)
	if err != nil {
		return nil, fmt.Errorf("Bleve vector channel ranking: %w", err)
	}
	ranks := make(map[string]int, len(result.Hits))
	for i, hit := range result.Hits {
		ranks[hit.ID] = i + 1
	}
	return ranks, nil
}
