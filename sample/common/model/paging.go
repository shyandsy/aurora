package model

// PagingReq 分页请求
type PagingReq struct {
	Page     int32 `form:"page"`
	PageSize int32 `form:"pageSize"`
}

// PagingResponse 分页响应
type PagingResponse struct {
	Page       int32       `json:"page"`
	PageSize   int32       `json:"pageSize"`
	Total      int64       `json:"total"`
	TotalPages int32       `json:"totalPages"`
	HasNext    bool        `json:"hasNext"`
	HasPrev    bool        `json:"hasPrev"`
	Items      interface{} `json:"items"`
}
