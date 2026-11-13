package service

import (
	"context"
	"time"
)

// EvalCase 是评测用例。
type EvalCase struct {
	Question     string `json:"question"`
	ExpectClause string `json:"expect_clause"`
	Answerable   bool   `json:"answerable"`
}

// RetrievalItem 是单条检索评测结果。
type RetrievalItem struct {
	Question     string  `json:"question"`
	ExpectClause string  `json:"expect_clause"`
	Rank         int     `json:"rank"`
	Hit          bool    `json:"hit"`
	TopClause    string  `json:"top_clause"`
	TopScore     float64 `json:"top_score"`
}

// RetrievalReport 是检索评测汇总。
type RetrievalReport struct {
	Total       int             `json:"total"`
	Hits        int             `json:"hits"`
	HitRate     float64         `json:"hit_rate"`
	MRR         float64         `json:"mrr"`
	TopK        int             `json:"top_k"`
	MinScore    float64         `json:"min_score"`
	EmbedModel  string          `json:"embed_model"`
	VectorImpl  string          `json:"vector_impl"`
	GeneratedAt time.Time       `json:"generated_at"`
	Items       []RetrievalItem `json:"items"`
}

// EvalRetrieval 统计 topK 命中率与 MRR，结果可落盘留档。
// 只统计有期望条款的用例；期望条款为空的用例属于“不可回答”，
// 由问答评测判定是否兜底。
func (s *Service) EvalRetrieval(ctx context.Context, cases []EvalCase, topK int) (*RetrievalReport, error) {
	if topK <= 0 {
		topK = s.Cfg.Retrieval.TopK
	}
	rep := &RetrievalReport{
		TopK:        topK,
		MinScore:    s.Cfg.Retrieval.MinScore,
		EmbedModel:  s.Embedder.Name(),
		VectorImpl:  s.Vector.Kind(),
		GeneratedAt: time.Now(),
	}
	for _, c := range cases {
		if c.ExpectClause == "" {
			continue
		}
		rep.Total++
		hits, err := s.Search(ctx, c.Question, topK, 0)
		if err != nil {
			return nil, err
		}
		item := RetrievalItem{Question: c.Question, ExpectClause: c.ExpectClause}
		if len(hits) > 0 {
			item.TopClause = hits[0].ClauseNo
			item.TopScore = hits[0].Score
		}
		for i, h := range hits {
			if h.ClauseNo == c.ExpectClause {
				item.Rank = i + 1
				item.Hit = true
				break
			}
		}
		if item.Hit {
			rep.Hits++
			rep.MRR += 1 / float64(item.Rank)
		}
		rep.Items = append(rep.Items, item)
	}
	if rep.Total > 0 {
		rep.HitRate = float64(rep.Hits) / float64(rep.Total)
		rep.MRR = rep.MRR / float64(rep.Total)
	}
	return rep, nil
}

// QAItem 是单条问答评测结果。
type QAItem struct {
	Question  string   `json:"question"`
	Expect    string   `json:"expect_clause"`
	Answered  bool     `json:"answered"`
	Correct   bool     `json:"correct"`
	Citations []string `json:"citations"`
	Answer    string   `json:"answer"`
	TopScore  float64  `json:"top_score"`
}

// QAReport 是问答评测汇总。
type QAReport struct {
	Total         int       `json:"total"`
	Answered      int       `json:"answered"`
	Correct       int       `json:"correct"`
	Fallback      int       `json:"fallback"`
	Wrong         int       `json:"wrong"`
	Mismatch      int       `json:"mismatch"`       // 可回答但引用错条款
	FalsePositive int       `json:"false_positive"` // 不可回答却被回答
	Accuracy      float64   `json:"accuracy"`
	Model         string    `json:"model"`
	GeneratedAt   time.Time `json:"generated_at"`
	Items         []QAItem  `json:"items"`
}

// EvalQA 用评测集跑完整问答链路：可回答的必须带正确条款号，
// 不可回答的必须走兜底，不允许出现编造条款。
func (s *Service) EvalQA(ctx context.Context, cases []EvalCase) (*QAReport, error) {
	rep := &QAReport{Total: len(cases), Model: s.Chat.Name(), GeneratedAt: time.Now()}
	for _, c := range cases {
		res, err := s.Ask(ctx, nil, c.Question, "")
		if err != nil {
			return nil, err
		}
		item := QAItem{
			Question: c.Question,
			Expect:   c.ExpectClause,
			Answered: res.Answered,
			TopScore: res.TopScore,
			Answer:   res.Answer,
		}
		for _, cit := range res.Citations {
			item.Citations = append(item.Citations, cit.ClauseNo)
		}
		if res.Answered {
			rep.Answered++
		} else {
			rep.Fallback++
		}

		if c.Answerable {
			ok := res.Answered
			if ok && c.ExpectClause != "" {
				ok = false
				for _, cit := range item.Citations {
					if cit == c.ExpectClause {
						ok = true
						break
					}
				}
			}
			if ok {
				rep.Correct++
				item.Correct = true
			} else {
				rep.Wrong++
				rep.Mismatch++
			}
		} else {
			// 不可回答的用例：走兜底且不编造条款号才算正确。
			if !res.Answered && len(res.Citations) == 0 {
				rep.Correct++
				item.Correct = true
			} else {
				rep.Wrong++
				rep.FalsePositive++
			}
		}
		rep.Items = append(rep.Items, item)
	}
	if rep.Total > 0 {
		rep.Accuracy = float64(rep.Correct) / float64(rep.Total)
	}
	return rep, nil
}
