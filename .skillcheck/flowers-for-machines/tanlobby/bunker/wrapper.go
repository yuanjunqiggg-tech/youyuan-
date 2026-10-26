package bunker

import "fmt"

// Authenticator ..
type Authenticator interface {
	GetAccess(roomID string) (TanLobbyLoginResponse, error)
	GetCreate() (TanLobbyCreateResponse, error)
	GetDebug(loginResponse string, raknetRand []byte) (TanLobbyDebugResponse, error)
}

// AccessWrapper ..
type AccessWrapper struct {
	client *Client
}

func NewAccessWrapper(authServer string, token string, peAuth string, saAuth string) *AccessWrapper {
	return &AccessWrapper{
		client: NewClient(authServer, token, peAuth, saAuth),
	}
}

// GetAccess ..
func (aw *AccessWrapper) GetAccess(roomID string) (TanLobbyLoginResponse, error) {
	tanLobbyLoginResp, err := aw.client.Auth(roomID)
	if err != nil {
		return TanLobbyLoginResponse{}, fmt.Errorf("获取访问失败：%v", err)
	}
	return tanLobbyLoginResp, nil
}

// GetCreate ..
func (aw *AccessWrapper) GetCreate() (TanLobbyCreateResponse, error) {
	tanLobbyCreateResp, err := aw.client.TanLobbyCreate()
	if err != nil {
		return TanLobbyCreateResponse{}, fmt.Errorf("获取创建失败：%v", err)
	}
	return tanLobbyCreateResp, nil
}

// GetCreate ..
func (aw *AccessWrapper) GetDebug(loginResponse string, raknetRand []byte) (TanLobbyDebugResponse, error) {
	tanLobbyDebugResp, err := aw.client.TanLobbyDebug(loginResponse, raknetRand)
	if err != nil {
		return TanLobbyDebugResponse{}, fmt.Errorf("获取调试失败：%v", err)
	}
	return tanLobbyDebugResp, nil
}
