package auth

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/pterm/pterm"

	I18n "github.com/OmineDev/flowers-for-machines/core/bunker/i18n"
)

type secretLoadingTransport struct {
	base   *http.Transport
	secret string
}

func (s secretLoadingTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	yyAttachDeviceKey(req)
	req.Header.Add("Authorization", fmt.Sprintf("Bearer %s", s.secret))
	if s.base != nil {
		return s.base.RoundTrip(req)
	}
	return http.DefaultTransport.RoundTrip(req)
}

func yyAttachDeviceKey(req *http.Request) {
	// Identify this client honestly. Cloudflare BIC may reject the generic Go
	// default; a product identifier is also useful for server-side diagnostics.
	if req.Header.Get("User-Agent") == "" {
		req.Header.Set("User-Agent", "YouYuan-Toolbox/0.10 (game-auth-client)")
	}
	if !(req.URL.Hostname() == "auth.youyuanqi.dpdns.org" || req.URL.Hostname() == "toolbox.youyuanqi.dpdns.org" && strings.HasPrefix(req.URL.Path, "/auth/")) {
		return
	}
	root := os.Getenv("YY_AUTH_CONFIG_DIR")
	if root == "" {
		return
	}
	data, err := os.ReadFile(root + "/youyuan-remote.json")
	var cfg struct {
		Token string `json:"device_token"`
	}
	if err == nil {
		_ = json.Unmarshal(data, &cfg)
	}
	if cfg.Token == "" && filepath.Base(filepath.Dir(root)) == "managed-accounts" {
		parentData, e := os.ReadFile(filepath.Join(filepath.Dir(filepath.Dir(root)), "youyuan-remote.json"))
		if e == nil {
			_ = json.Unmarshal(parentData, &cfg)
		}
	}
	if cfg.Token != "" {
		req.Header.Set("X-YouYuan-Device-Key", cfg.Token)
	}
}

type ClientOptions struct {
	AuthServer          string
	RespondUserOverride string
}

func MakeDefaultClientOptions() *ClientOptions {
	return &ClientOptions{}
}

type ClientInfo struct {
	GrowthLevel int
	RespondTo   string
}

type Client struct {
	ClientInfo
	client http.Client
	*ClientOptions
}

func parsedError(message string) error {
	var details map[string]json.RawMessage
	if json.Unmarshal([]byte(message), &details) == nil {
		for _, field := range []string{"message", "error"} {
			var reason string
			if json.Unmarshal(details[field], &reason) == nil && reason != "" {
				if len([]rune(reason)) > 300 {
					reason = string([]rune(reason)[:300])
				}
				return errors.New(reason)
			}
		}
	}
	error_regex := regexp.MustCompile("^(\\d{3} [a-zA-Z ]+)\n\n(.*?)($|\n)")
	err_matches := error_regex.FindAllStringSubmatch(message, 1)
	if len(err_matches) == 0 {
		return fmt.Errorf("未知错误")
	}
	return fmt.Errorf("%s: %s", err_matches[0][1], err_matches[0][2])
}

func assertAndParse[T any](resp *http.Response) (T, error) {
	var ret T
	defer resp.Body.Close()
	if resp.StatusCode == 503 {
		return ret, errors.New("API 服务器不可用")
	}
	_body, _ := io.ReadAll(resp.Body)
	body := string(_body)
	if resp.StatusCode != 200 {
		return ret, fmt.Errorf("认证 HTTP %d: %w", resp.StatusCode, parsedError(body))
	}
	err := json.Unmarshal([]byte(body), &ret)
	if err != nil {
		return ret, errors.New(fmt.Sprintf("解析 API 响应失败: %v", err))
	}
	return ret, nil
}

func CreateClient(options *ClientOptions) (*Client, error) {
	return CreateClientContext(context.Background(), options)
}

// CreateClientContext creates an auth-service session within the login lifecycle.
func CreateClientContext(ctx context.Context, options *ClientOptions) (*Client, error) {
	options.AuthServer = strings.TrimRight(options.AuthServer, "/")
	tr := &http.Transport{TLSClientConfig: &tls.Config{MinVersion: tls.VersionTLS12}}
	hc := http.Client{Transport: tr, Timeout: 25 * time.Second}
	if _, err := url.ParseRequestURI(options.AuthServer); err != nil {
		return nil, fmt.Errorf("认证地址无效")
	}
	req, err := http.NewRequestWithContext(ctx, "GET", fmt.Sprintf("%s/api/new", strings.TrimRight(options.AuthServer, "/")), nil)
	if err != nil {
		return nil, err
	}
	yyAttachDeviceKey(req)
	secret_res, err := hc.Do(req)
	if err != nil {
		return nil, authTransportError("new_session", err)
	}
	defer secret_res.Body.Close()
	_secret_body, err := io.ReadAll(secret_res.Body)
	if err != nil {
		return nil, authTransportError("new_session", err)
	}
	secret_body := string(_secret_body)
	if secret_res.StatusCode == 503 {
		return nil, errors.New("API 服务器不可用")
	} else if secret_res.StatusCode != 200 {
		return nil, fmt.Errorf("认证会话 HTTP %d: %w", secret_res.StatusCode, parsedError(secret_body))
	}
	authclient := &Client{
		client: http.Client{
			Transport: secretLoadingTransport{base: tr, secret: secret_body},
			// The owned provider may refresh a saved game session and then perform
			// official login/skin lookups. Bound the whole exchange without a
			// premature 15-second cutoff or automatic login replay.
			Timeout: 75 * time.Second,
		},
		ClientOptions: options,
		ClientInfo: ClientInfo{
			RespondTo: options.RespondUserOverride,
		},
	}
	return authclient, nil
}

// CreateClientWithSession reuses an existing private auth-service session.
// Gateways use this to avoid /api/new for each side of the same player login.
func CreateClientWithSession(options *ClientOptions, secret string) (*Client, error) {
	if secret == "" {
		return nil, fmt.Errorf("认证会话不能为空")
	}
	if _, err := url.ParseRequestURI(options.AuthServer); err != nil {
		return nil, fmt.Errorf("认证地址无效")
	}
	tr := &http.Transport{TLSClientConfig: &tls.Config{MinVersion: tls.VersionTLS12}}
	return &Client{client: http.Client{Transport: secretLoadingTransport{base: tr, secret: secret}, Timeout: 75 * time.Second}, ClientOptions: options}, nil
}

func authTransportError(phase string, err error) error {
	if errors.Is(err, context.Canceled) {
		return fmt.Errorf("认证已取消 phase=%s: %w", phase, context.Canceled)
	}
	var timeout interface{ Timeout() bool }
	if errors.Is(err, context.DeadlineExceeded) || errors.As(err, &timeout) && timeout.Timeout() {
		// A deadline after submission cannot prove that official authentication
		// did not finish. Never advise blindly replaying a game login.
		outcome := "unknown"
		if phase == "new_session" {
			outcome = "no_game_login_submitted"
		}
		return fmt.Errorf("认证请求超时 phase=%s outcome=%s；请检查状态后重试: %w", phase, outcome, context.DeadlineExceeded)
	}
	// Avoid embedding request URLs (which may contain scoped delegation grants)
	// or credentials from a transport's error string into user-visible logs.
	var requestError *url.Error
	if errors.As(err, &requestError) {
		return fmt.Errorf("认证网络请求失败 phase=%s: %w", phase, requestError.Err)
	}
	return fmt.Errorf("认证网络请求失败 phase=%s: %w", phase, err)
}

// 客户端向验证服务器发送的请求体，
// 用于获得 FBToken，
// 或使客户端登录到网易租赁服。
// AuthResponse 是该请求体对应的响应体
type AuthRequest struct {
	/*
		此字段非空时，则下方 UserName 和 Password 为空，
		否则反之。

		当 FBToken 或 UserName、Password 二者中任意一个
		填写的值正确时，用户将登录到用户中心，然后进入租赁服
	*/
	FBToken string `json:"login_token,omitempty"`

	UserName string `json:"username,omitempty"` // 用户在用户中心的用户名
	Password string `json:"password,omitempty"` // 用户在用户中心的密码

	ServerCode     string `json:"server_code"`     // 要进入的租赁服的 服务器号
	ServerPassword string `json:"server_passcode"` // 该租赁服的 密码

	ClientPublicKey string `json:"client_public_key"` // ...
}

// 验证服务器对 AuthRequest 的响应体
type AuthResponse struct {
	/*
		描述请求的成功状态。

		如果成功，则其余的所有非可选字段都将有值，
		这也包括 Message 本身。

		如果失败，除了本字段和 Message 以外，
		其余所有字段都为默认的零值，
		同时 Message 会展示对应的失败原因
	*/
	SuccessStates bool   `json:"success"`
	ServerMessage string `json:"server_msg,omitempty"` // 来自验证服务器的消息
	Message

	BotLevel     int             `json:"growth_level"`          // 机器人的等级
	BotSkin      SkinInfo        `json:"skin_info,omitempty"`   // 机器人的皮肤信息
	BotComponent map[string]*int `json:"outfit_info,omitempty"` // 机器人当前已加载的组件及其附加值

	FBToken    string `json:"token"`      // ...
	MasterName string `json:"respond_to"` // 机器人主人的游戏名称

	RentalServerIP string `json:"ip_address"` // 欲登录的租赁服的 IP 地址
	ChainInfo      string `json:"chainInfo"`  // 欲登录的租赁服的链请求
}

// 描述 AuthResponse 所附带的额外信息
type Message struct {
	/*
		若 AuthRequest 成功，
		则对于原生的 FastBuilder 的验证服务器(mv4)，
		此字段为 "正常返回"；
		否则，对于 咕咕酱及其开发团队 的验证服务器，
		此字段为 "well down"。

		当 AuthRequest 失败时，
		若此字段非空，则它将阐明对应的失败原因，
		否则，由下方的 Translation 揭示具体的原因
	*/
	Information string `json:"message,omitempty"`
	// 表示错误码，且可以与 i18n 中所记的映射对应。
	// 如果不存在，则其默认值为 0，
	// 如果未使用，则其默认值为 -1
	Translation int `json:"translation,omitempty"`
}

// 描述 AuthResponse 中附带的 机器人 的皮肤信息
type SkinInfo struct {
	ItemID          string `json:"entity_id"` // 皮肤的资源 ID
	SkinDownloadURL string `json:"res_url"`   // 皮肤的下载地址 [需要验证]
	SkinIsSlim      bool   `json:"is_slim"`   // 皮肤的手臂是否纤细
}

// Ret: chain, ip, token, error
func (client *Client) Auth(
	ctx context.Context,
	serverCode string, serverPassword string,
	key string,
	fbtoken string, username string, password string,
) (AuthResponse, error) {
	var authResponse AuthResponse
	// prepare
	request := AuthRequest{
		FBToken:         fbtoken,
		UserName:        username,
		Password:        password,
		ServerCode:      serverCode,
		ServerPassword:  serverPassword,
		ClientPublicKey: key,
	}
	authRequest, _ := json.Marshal(request)
	// pack request and marshal to binary
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, fmt.Sprintf("%s/api/phoenix/login", client.AuthServer), bytes.NewBuffer(authRequest))
	if err != nil {
		return authResponse, err
	}
	req.Header.Set("Content-Type", "application/json")
	httpResponse, err := client.client.Do(req)
	if err != nil {
		return authResponse, authTransportError("game_login", err)
	}
	authResponse, err = assertAndParse[AuthResponse](httpResponse)
	if err != nil {
		return authResponse, err
	}
	// get response
	if !authResponse.SuccessStates {
		failedReason := authResponse.Message
		err := failedReason.Information
		if t := failedReason.Translation; t != -1 && t != 0 {
			err = I18n.T(uint16(t))
		}
		return AuthResponse{}, fmt.Errorf("%s", err)
	}
	// if reuqest failed
	if len(authResponse.ServerMessage) > 0 {
		pterm.Println(pterm.LightGreen(I18n.T(I18n.Auth_MessageFromAuthServer)))
		pterm.Println(pterm.LightGreen(authResponse.ServerMessage))
	}
	// server message
	client.GrowthLevel = authResponse.BotLevel
	if len(authResponse.MasterName) > 0 && client.RespondTo == "" {
		client.RespondTo = authResponse.MasterName
	}
	// set value
	return authResponse, nil
	// return
}

func (client *Client) TransferData(content string) (string, error) {
	r, err := client.client.Get(fmt.Sprintf("%s/api/phoenix/transfer_start_type?content=%s", client.AuthServer, content))
	if err != nil {
		return "", err
	}
	resp, err := assertAndParse[map[string]any](r)
	if err != nil {
		return "", err
	}
	succ, _ := resp["success"].(bool)
	if !succ {
		err_m, _ := resp["message"].(string)
		return "", fmt.Errorf("传输起始类型失败: %s", err_m)
	}
	data, _ := resp["data"].(string)
	return data, nil
}

type FNumRequest struct {
	Data string `json:"data"`
}

func (client *Client) TransferCheckNum(data string) (string, error) {
	rspreq := &FNumRequest{
		Data: data,
	}
	msg, err := json.Marshal(rspreq)
	if err != nil {
		return "", errors.New("JSON 编码失败")
	}
	r, err := client.client.Post(fmt.Sprintf("%s/api/phoenix/transfer_check_num", client.AuthServer), "application/json", bytes.NewBuffer(msg))
	if err != nil {
		return "", err
	}
	resp, err := assertAndParse[map[string]any](r)
	if err != nil {
		return "", err
	}
	succ, _ := resp["success"].(bool)
	if !succ {
		err_m, _ := resp["message"].(string)
		return "", fmt.Errorf("传输校验数字失败: %s", err_m)
	}
	val, _ := resp["value"].(string)
	return val, nil
}

// ─── TanLobby (本地联机房间) ──────────────────────────────────────────

// hexDecode 将 hex 编码字符串解码为 []byte。
// 服务器返回的字节字段是 hex 编码，而 Go 默认 json.Unmarshal 对 []byte 按 base64 处理，
// 导致长度错误，因此需要手动 hex 解码。
func hexDecode(s string) []byte {
	if s == "" {
		return nil
	}
	b, err := hex.DecodeString(s)
	if err != nil {
		return []byte(s) // 解码失败时回退为原始字符串字节
	}
	return b
}

// hexDecodeSlice 将 hex 字符串切片解码为 [][]byte。
func hexDecodeSlice(ss []string) [][]byte {
	out := make([][]byte, 0, len(ss))
	for _, s := range ss {
		out = append(out, hexDecode(s))
	}
	return out
}

// TanLobbyLoginRequest 加入 TanLobby 房间的请求体。
type TanLobbyLoginRequest struct {
	FBToken string `json:"login_token"`
	RoomID  string `json:"room_id"`
}

// UnmarshalJSON 手动解码响应，将 hex 编码的字节字段转为 []byte。
func (r *TanLobbyLoginResponse) UnmarshalJSON(data []byte) error {
	// 用 string 类型接收字节字段，避免 Go 默认按 base64 解码 hex
	var tmp struct {
		Success   bool   `json:"success"`
		ErrorInfo string `json:"error_info"`

		UserUniqueID   uint32          `json:"user_unique_id"`
		UserPlayerName string          `json:"user_player_name"`
		BotLevel       int             `json:"growth_level"`
		BotSkin        SkinInfo        `json:"skin_info"`
		BotComponent   map[string]*int `json:"outfit_info,omitempty"`

		RoomOwnerID        uint32   `json:"room_owner_id"`
		RoomModDisplayName []string `json:"room_mod_display_name"`
		RoomModDownloadURL []string `json:"room_mod_download_url"`
		RoomModEncryptKey  []string `json:"room_mod_encrypt_key"`

		RaknetServerAddress string `json:"raknet_server_address"`
		RaknetRand          string `json:"raknet_rand"`
		RaknetAESRand       string `json:"raknet_aes_rand"`
		EncryptKeyBytes     string `json:"encrypt_key_bytes"`
		DecryptKeyBytes     string `json:"decrypt_key_bytes"`

		SignalingServerAddress string `json:"signaling_server_address"`
		SignalingSeed          string `json:"signaling_seed"`
		SignalingTicket        string `json:"signaling_ticket"`
	}
	if err := json.Unmarshal(data, &tmp); err != nil {
		return err
	}

	r.Success = tmp.Success
	r.ErrorInfo = tmp.ErrorInfo
	r.UserUniqueID = tmp.UserUniqueID
	r.UserPlayerName = tmp.UserPlayerName
	r.BotLevel = tmp.BotLevel
	r.BotSkin = tmp.BotSkin
	r.BotComponent = tmp.BotComponent
	r.RoomOwnerID = tmp.RoomOwnerID
	r.RoomModDisplayName = tmp.RoomModDisplayName
	r.RoomModDownloadURL = tmp.RoomModDownloadURL
	r.RoomModEncryptKey = hexDecodeSlice(tmp.RoomModEncryptKey)
	r.RaknetServerAddress = tmp.RaknetServerAddress
	r.RaknetRand = hexDecode(tmp.RaknetRand)
	r.RaknetAESRand = hexDecode(tmp.RaknetAESRand)
	r.EncryptKeyBytes = hexDecode(tmp.EncryptKeyBytes)
	r.DecryptKeyBytes = hexDecode(tmp.DecryptKeyBytes)
	r.SignalingServerAddress = tmp.SignalingServerAddress
	r.SignalingSeed = hexDecode(tmp.SignalingSeed)
	r.SignalingTicket = hexDecode(tmp.SignalingTicket)
	return nil
}

// TanLobbyLoginResponse 加入 TanLobby 房间的响应体。
type TanLobbyLoginResponse struct {
	Success   bool   `json:"success"`
	ErrorInfo string `json:"message"`

	UserUniqueID   uint32          `json:"user_unique_id"`
	UserPlayerName string          `json:"user_player_name"`
	BotLevel       int             `json:"growth_level"`
	BotSkin        SkinInfo        `json:"skin_info"`
	BotComponent   map[string]*int `json:"outfit_info,omitempty"`

	RoomOwnerID        uint32   `json:"room_owner_id"`
	RoomModDisplayName []string `json:"room_mod_display_name"`
	RoomModDownloadURL []string `json:"room_mod_download_url"`
	RoomModEncryptKey  [][]byte `json:"room_mod_encrypt_key"`

	RaknetServerAddress string `json:"raknet_server_address"`
	RaknetRand          []byte `json:"raknet_rand"`
	RaknetAESRand       []byte `json:"raknet_aes_rand"`
	EncryptKeyBytes     []byte `json:"encrypt_key_bytes"`
	DecryptKeyBytes     []byte `json:"decrypt_key_bytes"`

	SignalingServerAddress string `json:"signaling_server_address"`
	SignalingSeed          []byte `json:"signaling_seed"`
	SignalingTicket        []byte `json:"signaling_ticket"`
}

// TanLobbyCreateRequest 创建 TanLobby 房间的请求体。
type TanLobbyCreateRequest struct {
	FBToken string `json:"login_token"`
}

// TanLobbyCreateResponse 创建 TanLobby 房间的响应体。
type TanLobbyCreateResponse struct {
	Success   bool   `json:"success"`
	ErrorInfo string `json:"message"`

	UserUniqueID   uint32 `json:"user_unique_id"`
	UserPlayerName string `json:"user_player_name"`

	RaknetServerAddress string `json:"raknet_server_address"`
	RaknetRand          []byte `json:"raknet_rand"`
	RaknetAESRand       []byte `json:"raknet_aes_rand"`
	EncryptKeyBytes     []byte `json:"encrypt_key_bytes"`
	DecryptKeyBytes     []byte `json:"decrypt_key_bytes"`

	SignalingServerAddress string `json:"signaling_server_address"`
	SignalingSeed          []byte `json:"signaling_seed"`
	SignalingTicket        []byte `json:"signaling_ticket"`
}

// UnmarshalJSON 手动解码响应，将 hex 编码的字节字段转为 []byte。
func (r *TanLobbyCreateResponse) UnmarshalJSON(data []byte) error {
	var tmp struct {
		Success   bool   `json:"success"`
		ErrorInfo string `json:"error_info"`

		UserUniqueID   uint32 `json:"user_unique_id"`
		UserPlayerName string `json:"user_player_name"`

		RaknetServerAddress string `json:"raknet_server_address"`
		RaknetRand          string `json:"raknet_rand"`
		RaknetAESRand       string `json:"raknet_aes_rand"`
		EncryptKeyBytes     string `json:"encrypt_key_bytes"`
		DecryptKeyBytes     string `json:"decrypt_key_bytes"`

		SignalingServerAddress string `json:"signaling_server_address"`
		SignalingSeed          string `json:"signaling_seed"`
		SignalingTicket        string `json:"signaling_ticket"`
	}
	if err := json.Unmarshal(data, &tmp); err != nil {
		return err
	}

	r.Success = tmp.Success
	r.ErrorInfo = tmp.ErrorInfo
	r.UserUniqueID = tmp.UserUniqueID
	r.UserPlayerName = tmp.UserPlayerName
	r.RaknetServerAddress = tmp.RaknetServerAddress
	r.RaknetRand = hexDecode(tmp.RaknetRand)
	r.RaknetAESRand = hexDecode(tmp.RaknetAESRand)
	r.EncryptKeyBytes = hexDecode(tmp.EncryptKeyBytes)
	r.DecryptKeyBytes = hexDecode(tmp.DecryptKeyBytes)
	r.SignalingServerAddress = tmp.SignalingServerAddress
	r.SignalingSeed = hexDecode(tmp.SignalingSeed)
	r.SignalingTicket = hexDecode(tmp.SignalingTicket)
	return nil
}

// TanLobbyAuth 通过认证服务器加入 TanLobby 房间，返回连接所需的中继/信令信息。
func (client *Client) TanLobbyAuth(roomID string, fbtoken string) (TanLobbyLoginResponse, error) {
	return client.TanLobbyAuthContext(context.Background(), roomID, fbtoken)
}

// TanLobbyAuthContext allows cancellation of a submitted room-auth request.
func (client *Client) TanLobbyAuthContext(ctx context.Context, roomID string, fbtoken string) (TanLobbyLoginResponse, error) {
	request := TanLobbyLoginRequest{
		FBToken: fbtoken,
		RoomID:  roomID,
	}
	requestJSONBytes, _ := json.Marshal(request)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, fmt.Sprintf("%s/api/phoenix/tan_lobby_login", client.AuthServer), bytes.NewBuffer(requestJSONBytes))
	if err != nil {
		return TanLobbyLoginResponse{}, fmt.Errorf("TanLobby 登录失败: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	httpResponse, err := client.client.Do(req)
	if err != nil {
		return TanLobbyLoginResponse{}, authTransportError("tan_lobby_login", err)
	}
	response, err := assertAndParse[TanLobbyLoginResponse](httpResponse)
	if err != nil {
		if ctx.Err() != nil {
			return TanLobbyLoginResponse{}, authTransportError("tan_lobby_login", ctx.Err())
		}
		return TanLobbyLoginResponse{}, fmt.Errorf("TanLobby 登录失败: %w", err)
	}
	// 服务器失败时 HTTP 仍返回 200，需检查 success 字段
	if !response.Success {
		return TanLobbyLoginResponse{}, fmt.Errorf("TanLobby 登录失败: %s", response.ErrorInfo)
	}
	return response, nil
}

// TanLobbyCreate 通过认证服务器创建 TanLobby 房间。
func (client *Client) TanLobbyCreate(fbtoken string) (TanLobbyCreateResponse, error) {
	request := TanLobbyCreateRequest{FBToken: fbtoken}
	requestJSONBytes, _ := json.Marshal(request)
	httpResponse, err := client.client.Post(
		fmt.Sprintf("%s/api/phoenix/tan_lobby_create", client.AuthServer),
		"application/json",
		bytes.NewBuffer(requestJSONBytes),
	)
	if err != nil {
		return TanLobbyCreateResponse{}, fmt.Errorf("创建 TanLobby 房间失败: %v", err)
	}
	response, err := assertAndParse[TanLobbyCreateResponse](httpResponse)
	if err != nil {
		return TanLobbyCreateResponse{}, fmt.Errorf("创建 TanLobby 房间失败: %v", err)
	}
	// 服务器失败时 HTTP 仍返回 200，需检查 success 字段
	if !response.Success {
		return TanLobbyCreateResponse{}, fmt.Errorf("创建 TanLobby 房间失败: %s", response.ErrorInfo)
	}
	return response, nil
}
