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

type BrunchOp string

const (
	BrunchOpCreate BrunchOp = "create"
	BrunchOpUpdate BrunchOp = "update"
	BrunchOpDelete BrunchOp = "delete"
)

type BrunchAdminRequest struct {
	SecretKey string `json:"key"`
	Op        BrunchOp
	Username  string `json:"username"`
	Password  string `json:"password"`
}

type BrunchAdminResponse struct {
	Code int `json:"code"`
}

type BrunchQueryRequest struct {
	Token string `json:"token"`
	Op    BrunchOp
	Key   string `json:"key"`
	Value string `json:"value,omitempty"`
}

type BrunchQueryResponse struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Result  string `json:"result"`
}
