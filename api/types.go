package api

type ApiClient struct {
	token      string
	skipVerify bool
	https      bool
}

type BrunchAuthRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type BrunchAuthResponse struct {
	Token   string `json:"token"`
	Code    int    `json:"code"`
	Message string `json:"message"`
}

type BrunchQueryRequest struct {
	Token string `json:"token"`
	Query string `json:"query"`
}

type BrunchQueryResponse struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Result  string `json:"result"`
}

type BrunchUserOp string

const (
	BranchOpCreate BrunchUserOp = "create"
	BranchOpUpdate BrunchUserOp = "update"
	BranchOpDelete BrunchUserOp = "delete"
)

type BrunchAdminRequest struct {
	SecretKey string `json:"key"`
	Op        BrunchUserOp
	Username  string `json:"username"`
	Password  string `json:"password"`
}

type BrunchAdminResponse struct {
	Code int `json:"code"`
}
