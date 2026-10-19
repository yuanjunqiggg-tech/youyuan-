package minecraft

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	cryptoRand "crypto/rand"
	"crypto/x509"
	_ "embed"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"image/png"
	"io"
	"log/slog"
	"math"
	mathRand "math/rand"
	"net"
	"net/http"
	"os"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/OmineDev/flowers-for-machines/core/bunker/auth"
	"github.com/OmineDev/flowers-for-machines/core/minecraft/internal"
	"github.com/OmineDev/flowers-for-machines/core/minecraft/protocol"
	"github.com/OmineDev/flowers-for-machines/core/minecraft/protocol/login"
	"github.com/OmineDev/flowers-for-machines/core/minecraft/protocol/packet"
	"github.com/OmineDev/flowers-for-machines/core/minecraft/raknet"

	"github.com/go-jose/go-jose/v4"
	"github.com/go-jose/go-jose/v4/jwt"
	"github.com/google/uuid"
)

// PhoenixBuilder specific interface.
// Author: LNSSPsd, Liliya233, Happy2018new
type Authenticator interface {
	GetAccess(ctx context.Context, publicKey []byte) (auth.AuthResponse, error)
}

// Dialer allows specifying specific settings for connection to a Minecraft server.
// The zero value of Dialer is used for the package level Dial function.
type Dialer struct {
	// UpstreamDialer routes RakNet UDP without changing authentication/identity.
	UpstreamDialer raknet.UpstreamDialer
	// DisablePharos preserves the authenticated server destination unchanged.
	DisablePharos bool
	// OnGameRoute reports whether an official Pharos candidate was selected.
	OnGameRoute func(officialSelected bool)
	// LoginOnly is for same-protocol gateways. Complete authentication and encryption,
	// then expose resource packs and all world packets through ReadBytes without
	// automatically acting as a game client. Normal bot sessions leave this false.
	LoginOnly bool
	// PreserveClientData retains the routed client's platform and skin. Only use
	// this with actual client data and a new key-bound chain for the same identity.
	PreserveClientData bool
	// ErrorLog is a log.Logger that errors that occur during packet handling of
	// servers are written to. By default, errors are not logged.
	ErrorLog *slog.Logger

	// ClientData is the client data used to login to the server with. It includes fields such as the skin,
	// locale and UUIDs unique to the client. If empty, a default is sent produced using defaultClientData().
	ClientData login.ClientData
	// IdentityData is the identity data used to login to the server with. It includes the username, UUID and
	// XUID of the player.
	// The IdentityData object is obtained using Minecraft auth if Email and Password are set. If not, the
	// object provided here is used, or a default one if left empty.
	IdentityData login.IdentityData

	// PhoenixBuilder specific changes.
	// Author: LNSSPsd
	//
	// Authenticator towards netease's server
	Authenticator

	// PacketFunc is called whenever a packet is read from or written to the connection returned when using
	// Dialer.Dial(). It includes packets that are otherwise covered in the connection sequence, such as the
	// Login packet. The function is called with the header of the packet and its raw payload, the address
	// from which the packet originated, and the destination address.
	PacketFunc func(header packet.Header, payload []byte, src, dst net.Addr)

	// DownloadResourcePack is called individually for every texture and behaviour pack sent by the connection when
	// using Dialer.Dial(), and can be used to stop the pack from being downloaded. The function is called with the UUID
	// and version of the resource pack, the number of the current pack being downloaded, and the total amount of packs.
	// The boolean returned determines if the pack will be downloaded or not.
	DownloadResourcePack func(id uuid.UUID, version string, current, total int) bool

	// DisconnectOnUnknownPackets specifies if the connection should disconnect if packets received are not present
	// in the packet pool. If true, such packets lead to the connection being closed immediately.
	// If set to false, the packets will be returned as a packet.Unknown.
	DisconnectOnUnknownPackets bool

	// DisconnectOnInvalidPackets specifies if invalid packets (either too few bytes or too many bytes) should be
	// allowed. If true, such packets lead to the connection being closed immediately. If false,
	// packets with too many bytes will be returned while packets with too few bytes will be skipped.
	DisconnectOnInvalidPackets bool

	// Protocol is the Protocol version used to communicate with the target server. By default, this field is
	// set to the current protocol as implemented in the minecraft/protocol package. Note that packets written
	// to and read from the Conn are always any of those found in the protocol/packet package, as packets
	// are converted from and to this Protocol.
	Protocol Protocol

	// FlushRate is the rate at which packets sent are flushed. Packets are buffered for a duration up to
	// FlushRate and are compressed/encrypted together to improve compression ratios. The lower this
	// time.Duration, the lower the latency but the less efficient both network and cpu wise.
	// The default FlushRate (when set to 0) is time.Second/20. If FlushRate is set negative, packets
	// will not be flushed automatically. In this case, calling `(*Conn).Flush()` is required after any
	// calls to `(*Conn).Write()` or `(*Conn).WritePacket()` to send the packets over network.
	FlushRate time.Duration

	// EnableClientCache, if set to true, enables the client blob cache for the client. This means that the
	// server will send chunks as blobs, which may be saved by the client so that chunks don't have to be
	// transmitted every time, resulting in less network transmission.
	EnableClientCache bool

	// KeepXBLIdentityData, if set to true, enables passing XUID and title ID to the target server
	// if the authentication token is not set. This is technically not valid and some servers might kick
	// the client when an XUID is present without logging in.
	// For getting this to work with BDS, authentication should be disabled.
	KeepXBLIdentityData bool

	// EnableLegacyAuth, if set to true, will use the legacy authentication behavior
	// (pre-1.21.90) when connecting to the server. This should only be used for outdated
	// servers, as enabling it will cause compatibility issues with updated servers.
	EnableLegacyAuth bool
}

/*
PhoenixBuilder specific changes.
Author: Happy2018new

Dial dials a Minecraft connection to the address passed over the network passed. The network is typically
"raknet". A Conn is returned which may be used to receive packets from and send packets to.

A zero value of a Dialer struct is used to initiate the connection. A custom Dialer may be used to specify
additional behaviour.
*/
func Dial(network string) (*Conn, auth.AuthResponse, error) {
	// func Dial(network, address string) (*Conn, error) {
	var d Dialer
	return d.Dial(network)
}

// PhoenixBuilder specific changes.
// Author: Happy2018new
//
// DialTimeout dials a Minecraft connection to the address passed over the network passed. The network is
// typically "raknet". A Conn is returned which may be used to receive packets from and send packets to.
// If a connection is not established before the timeout ends, DialTimeout returns an error.
// DialTimeout uses a zero value of Dialer to initiate the connection.
func DialTimeout(network string, timeout time.Duration) (*Conn, auth.AuthResponse, error) {
	// DialTimeout(network, address string, timeout time.Duration) (*Conn, error)
	var d Dialer
	return d.DialTimeout(network, timeout)
}

// PhoenixBuilder specific changes.
// Author: Happy2018new
//
// DialContext dials a Minecraft connection to the address passed over the network passed. The network is
// typically "raknet". A Conn is returned which may be used to receive packets from and send packets to.
// If a connection is not established before the context passed is cancelled, DialContext returns an error.
// DialContext uses a zero value of Dialer to initiate the connection.
func DialContext(ctx context.Context, network string) (*Conn, auth.AuthResponse, error) {
	// func DialContext(ctx context.Context, network, address string) (*Conn, error) {
	var d Dialer
	return d.DialContext(ctx, network)
}

// PhoenixBuilder specific changes.
// Author: Happy2018new
//
// Dial dials a Minecraft connection to the address passed over the network passed. The network is typically
// "raknet". A Conn is returned which may be used to receive packets from and send packets to.
func (d Dialer) Dial(network string) (*Conn, auth.AuthResponse, error) {
	// func (d Dialer) Dial(network, address string) (*Conn, error) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*30)
	defer cancel()
	return d.DialContext(ctx, network)
}

// PhoenixBuilder specific changes.
// Author: Happy2018new
//
// DialTimeout dials a Minecraft connection to the address passed over the network passed. The network is
// typically "raknet". A Conn is returned which may be used to receive packets from and send packets to.
// If a connection is not established before the timeout ends, DialTimeout returns an error.
func (d Dialer) DialTimeout(network string, timeout time.Duration) (*Conn, auth.AuthResponse, error) {
	// func (d Dialer) DialTimeout(network, address string, timeout time.Duration) (*Conn, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	return d.DialContext(ctx, network)
}

// PhoenixBuilder specific func, which modified from orgin version.
// Author: LNSSPsd, CMA2401PT, Liliya233, Happy2018new
//
// DialContext dials a Minecraft connection to the address passed over the network passed. The network is
// typically "raknet". A Conn is returned which may be used to receive packets from and send packets to.
// If a connection is not established before the context passed is cancelled, DialContext returns an error.
func (d Dialer) DialContext(ctx context.Context, network string) (conn *Conn, authResponse auth.AuthResponse, err error) {
	key, err := ecdsa.GenerateKey(elliptic.P384(), cryptoRand.Reader)
	if err != nil {
		return nil, auth.AuthResponse{}, &net.OpError{Op: "dial", Net: "minecraft", Err: fmt.Errorf("生成 ECDSA 密钥失败：%w", err)}
	}
	armoured_key, err := x509.MarshalPKIXPublicKey(&key.PublicKey)
	if err != nil {
		return nil, auth.AuthResponse{}, &net.OpError{Op: "dial", Net: "minecraft", Err: fmt.Errorf("序列化 PKIX 公钥失败：%w", err)}
	}

	authResponse, err = d.Authenticator.GetAccess(ctx, armoured_key)
	if err != nil {
		return nil, auth.AuthResponse{}, err
	}
	if d.ErrorLog == nil {
		d.ErrorLog = slog.New(internal.DiscardHandler{})
	}
	d.ErrorLog = d.ErrorLog.With("src", "dialer")
	if d.Protocol == nil {
		d.Protocol = DefaultProtocol
	}
	if d.FlushRate == 0 {
		d.FlushRate = time.Second / 20
	}

	n, ok := networkByID(network, d.ErrorLog)
	if !ok {
		return nil, auth.AuthResponse{}, &net.OpError{Op: "dial", Net: "minecraft", Err: fmt.Errorf("无可用网络：未找到 id %v 对应的网络", network)}
	}
	if d.UpstreamDialer != nil {
		if network != "raknet" {
			return nil, auth.AuthResponse{}, fmt.Errorf("configured game UDP egress does not support network %s", network)
		}
		n = RakNet{l: d.ErrorLog, upstream: d.UpstreamDialer}
	}

	/*
		Delete by Liliya233.

		var pong []byte
		var netConn net.Conn
		if pong, err = n.PingContext(ctx, address); err == nil {
			netConn, err = n.DialContext(ctx, addressWithPongPort(pong, address))
		} else {
			netConn, err = n.DialContext(ctx, address)
		}
	*/

	// PhoenixBuilder specific changes.
	// Author: CoozillaX
	//
	// Netease: pharos speed up
	var netConn net.Conn
	officialSelected := false
	if !d.DisablePharos {
		if addr, err := getPharosSpeedUpIPContext(ctx, authResponse.RentalServerIP); err == nil {
			authResponse.RentalServerIP = addr
			officialSelected = true
		}
	}
	if d.OnGameRoute != nil {
		d.OnGameRoute(officialSelected)
	}
	netConn, err = n.DialContext(ctx, authResponse.RentalServerIP)
	if err != nil {
		return nil, auth.AuthResponse{}, err
	}

	conn = newConn(netConn, key, d.ErrorLog, d.Protocol, d.FlushRate, false)
	conn.loginOnly = d.LoginOnly
	conn.pool = conn.proto.Packets(false)
	conn.identityData = d.IdentityData
	conn.clientData = d.ClientData
	conn.packetFunc = d.PacketFunc
	conn.downloadResourcePack = d.DownloadResourcePack
	conn.cacheEnabled = d.EnableClientCache
	conn.disconnectOnInvalidPacket = d.DisconnectOnInvalidPackets
	conn.disconnectOnUnknownPacket = d.DisconnectOnUnknownPackets
	conn.maxDecompressedLen = math.MaxInt

	defaultIdentityData(&conn.identityData)
	if d.PreserveClientData {
		conn.clientData.ServerAddress = authResponse.RentalServerIP
	} else {
		defaultClientData(&conn.clientData, authResponse)
	}

	var request []byte
	// We login as an Android device and this will show up in the 'titleId' field in the JWT chain, which
	// we can't edit. We just enforce Android data for logging in.
	if !d.PreserveClientData {
		setAndroidData(&conn.clientData)
	}

	request = login.Encode(authResponse.ChainInfo, conn.clientData, key, d.EnableLegacyAuth)
	identityData, _, _, err := login.Parse(request)
	if err != nil {
		if d.LoginOnly {
			_ = conn.Close()
			return nil, auth.AuthResponse{}, fmt.Errorf("代理登录链校验失败：%w", err)
		}
		fmt.Printf("WARNING: Identity data parsing error: %v\n", err)
	}
	// If we got the identity data from Minecraft auth, we need to make sure we set it in the Conn too, as
	// we are not aware of the identity data ourselves yet.
	conn.identityData = identityData

	readyForLogin, connected := make(chan struct{}), make(chan struct{})
	ctx, cancel := context.WithCancelCause(ctx)
	go listenConn(conn, readyForLogin, connected, cancel)

	conn.expect(packet.IDNetworkSettings, packet.IDPlayStatus)
	if err := conn.WritePacket(&packet.RequestNetworkSettings{ClientProtocol: d.Protocol.ID()}); err != nil {
		return nil, auth.AuthResponse{}, conn.wrap(fmt.Errorf("发送网络设置请求失败：%w", err), "dial")
	}
	_ = conn.Flush()

	select {
	case <-ctx.Done():
		return nil, auth.AuthResponse{}, conn.wrap(context.Cause(ctx), "dial")
	case <-conn.ctx.Done():
		return nil, auth.AuthResponse{}, conn.closeErr("dial")
	case <-readyForLogin:
		// We've received our network settings, so we can now send our login request.
		conn.expect(packet.IDServerToClientHandshake, packet.IDPlayStatus)
		if err := conn.WritePacket(&packet.Login{ConnectionRequest: request, ClientProtocol: d.Protocol.ID()}); err != nil {
			return nil, auth.AuthResponse{}, conn.wrap(fmt.Errorf("发送登录请求失败：%w", err), "dial")
		}
		_ = conn.Flush()

		select {
		case <-ctx.Done():
			return nil, auth.AuthResponse{}, conn.wrap(context.Cause(ctx), "dial")
		case <-conn.ctx.Done():
			return nil, auth.AuthResponse{}, conn.closeErr("dial")
		case <-connected:
			// We've connected successfully. We return the connection and no error.
			return conn, authResponse, nil
		}
	}
}

// readChainIdentityData reads a login.IdentityData from the Mojang chain
// obtained through authentication.
func readChainIdentityData(chainData []byte) (login.IdentityData, error) {
	chain := struct{ Chain []string }{}
	if err := json.Unmarshal(chainData, &chain); err != nil {
		return login.IdentityData{}, fmt.Errorf("读取登录链失败：解析 JSON：%w", err)
	}
	data := chain.Chain[1]
	claims := struct {
		ExtraData login.IdentityData `json:"extraData"`
	}{}
	tok, err := jwt.ParseSigned(data, []jose.SignatureAlgorithm{jose.ES384})
	if err != nil {
		return login.IdentityData{}, fmt.Errorf("读取登录链失败：解析 JWT：%w", err)
	}
	if err := tok.UnsafeClaimsWithoutVerification(&claims); err != nil {
		return login.IdentityData{}, fmt.Errorf("读取登录链失败：读取声明：%w", err)
	}
	if claims.ExtraData.Identity == "" {
		return login.IdentityData{}, fmt.Errorf("读取登录链失败：未找到附加数据")
	}
	return claims.ExtraData, nil
}

// listenConn listens on the connection until it is closed on another goroutine. The channel passed will
// receive a value once the connection is logged in.
func listenConn(conn *Conn, readyForLogin, connected chan struct{}, cancel context.CancelCauseFunc) {
	defer func() {
		_ = conn.Close()
	}()
	cancelContext := true
	for {
		// We finally arrived at the packet decoding loop. We constantly decode packets that arrive
		// and push them to the Conn so that they may be processed.
		packets, err := conn.dec.Decode()
		if err != nil {
			if !errors.Is(err, net.ErrClosed) {
				if cancelContext {
					cancel(err)
				} else {
					conn.log.Error(err.Error())
				}
			}
			return
		}
		for _, data := range packets {
			loggedInBefore, readyToLoginBefore := conn.loggedIn, conn.readyToLogin
			if err := conn.receive(data); err != nil {
				if cancelContext {
					cancel(err)
				} else {
					conn.log.Error(err.Error())
				}
				return
			}
			if !readyToLoginBefore && conn.readyToLogin {
				// This is the signal that the connection is ready to login, so we put a value in the channel so that
				// it may be detected.
				readyForLogin <- struct{}{}
			}
			if !loggedInBefore && conn.loggedIn {
				// This is the signal that the connection was considered logged in, so we put a value in the channel so
				// that it may be detected.
				cancelContext = false
				connected <- struct{}{}
			}
		}
	}
}

//go:embed skin_resource_patch.json
var skinResourcePatch []byte

//go:embed skin_geometry.json
var skinGeometry []byte

// PhoenixBuilder specific changes.
// Author: Happy2018new
//
// defaultClientData edits the ClientData passed to have defaults set to all fields that were left unchanged.
func defaultClientData(
	// PhoenixBuilder specific changes.
	// Author: Liliya233, Happy2018new
	d *login.ClientData,
	authResponse auth.AuthResponse,
	// address, username string, d *login.ClientData,
) {
	d.ServerAddress = authResponse.RentalServerIP
	d.DeviceOS = protocol.DeviceAndroid
	d.DefaultInputMode = packet.InputModeTouch
	d.CurrentInputMode = packet.InputModeTouch
	d.GameVersion = protocol.CurrentVersion
	d.ClientRandomID = mathRand.Int63()
	d.DeviceID = uuid.New().String()
	d.LanguageCode = "zh_CN" // Netease
	d.AnimatedImageData = make([]login.SkinAnimation, 0)
	d.PersonaPieces = make([]login.PersonaPiece, 0)
	d.PieceTintColours = make([]login.PersonaPieceTintColour, 0)
	d.SelfSignedID = uuid.New().String()
	d.SkinID = uuid.New().String()
	d.ArmSize = "wide"
	d.SkinColour = "#b37b62"
	d.PremiumSkin = false
	d.PersonaSkin = false
	d.TrustedSkin = true
	d.OverrideSkin = true
	d.UIProfile = 0
	d.CapeOnClassicSkin = false
	d.CompatibleWithClientSideChunkGen = true
	d.IsEditorMode = false
	d.GrowthLevel = authResponse.BotLevel
	// 补全官方 Android 客户端的设备字段：网易可能据此识别/信任客户端，
	// 缺失这些字段会让服务器将客户端皮肤标记为不可信而不应用。
	d.DeviceModel = "Xiaomi 21091116AC"
	d.ThirdPartyName = authResponse.MasterName
	d.MaxViewDistance = 10
	d.MemoryTier = 1
	d.PlatformType = 1 // UWP
	d.GraphicsMode = 0
	d.PlatformOfflineID = d.SelfSignedID
	d.PlatformOnlineID = ""
	// 与 NexusEgo 对齐：SkinItemID（entity_id）随登录 JWT ClientData 发送给服务器
	d.SkinItemID = authResponse.BotSkin.ItemID
	// 与 NexusEgo 对齐：设置 SkinGeometryVersion
	d.SkinGeometryVersion = base64.StdEncoding.EncodeToString([]byte("0.0.0"))

	// 从验证服务器下载皮肤，失败时回退到内置默认皮肤
	skinLoaded := false
	if authResponse.BotSkin.SkinDownloadURL != "" {
		url := authResponse.BotSkin.SkinDownloadURL
		isSlim := authResponse.BotSkin.SkinIsSlim
		skinDataStr, geoB64, patchB64, w, h, skinUUID, err := loadBotSkin(url, isSlim)
		if err != nil {
			fmt.Fprintf(os.Stderr, "[skin] 下载失败 %s: %v，回退到默认皮肤\n", url, err)
		} else {
			d.SkinData = skinDataStr
			d.SkinGeometry = geoB64
			d.SkinResourcePatch = patchB64
			d.SkinImageWidth = w
			d.SkinImageHeight = h
			// 高级皮肤（ZIP）携带 manifest UUID，用作 SkinID
			if skinUUID != "" {
				d.SkinID = skinUUID
			}
			// 统一宽臂：网易客户端对 slim 条目不渲染（实测）
			d.ArmSize = "wide"
			skinLoaded = true
			fmt.Fprintf(os.Stderr, "[skin] 已加载皮肤: %dx%d slim=%v skinUUID=%s itemID=%s url=%s\n",
				w, h, isSlim, skinUUID, authResponse.BotSkin.ItemID, url)
		}
	}
	if !skinLoaded {
		// 内置默认皮肤 (64x32 Steve 风格 RGBA)
		d.SkinData = base64.StdEncoding.EncodeToString(generateDefaultSkinRGBA())
		d.SkinImageWidth = 64
		d.SkinImageHeight = 32
		d.SkinGeometry = base64.StdEncoding.EncodeToString(skinGeometry)
		d.SkinResourcePatch = base64.StdEncoding.EncodeToString(skinResourcePatch)
	}
	// 与 NexusEgo 对齐：皮肤下载成功时 PremiumSkin=true，失败时 false
	d.PremiumSkin = skinLoaded

	// 与真实网易客户端对齐（观察自真实玩家 PlayerList 广播条目）：
	// - 自定义皮肤 SkinID 格式为 "<基础UUID>.Custom<去横线随机UUID>"，
	//   .Custom 标记让接收方客户端使用包内原始皮肤数据；
	//   普通 UUID 会被当作皮肤商店条目，查不到则回退默认皮肤不渲染。
	// - CapeID 字段被网易复用为皮肤 item id（真实条目 CapeID=皮肤item_id，
	//   服务器据此拼接 FullID = SkinID + CapeID）。
	customID := strings.ReplaceAll(uuid.New().String(), "-", "")
	baseSkinID := "c18e65aa-7b21-4637-9b63-8ad63622ef01"
	if skinLoaded && d.SkinID != "" {
		baseSkinID = strings.Split(d.SkinID, ".Custom")[0]
	}
	d.SkinID = baseSkinID + ".Custom" + customID
	d.CapeID = authResponse.BotSkin.ItemID
	// 对齐真实客户端：无肤色、不覆盖本地皮肤
	d.SkinColour = "#0"
	d.OverrideSkin = false

	{
		id := make([]byte, 8)
		_, _ = cryptoRand.Read(id)
		d.PlayFabID = hex.EncodeToString(id)
	}
}

// setAndroidData ensures the login.ClientData passed matches settings you would see on an Android device.
func setAndroidData(data *login.ClientData) {
	data.DeviceOS = protocol.DeviceAndroid
	data.GameVersion = protocol.CurrentVersion
}

// clearXBLIdentityData clears data from the login.IdentityData that is only set when a player is logged into
// XBOX Live.
func clearXBLIdentityData(data *login.IdentityData) {
	data.XUID = ""
	data.TitleID = ""
}

// defaultIdentityData edits the IdentityData passed to have defaults set to all fields that were left
// unchanged.
func defaultIdentityData(data *login.IdentityData) {
	if data.Identity == "" {
		data.Identity = uuid.New().String()
	}
	if data.DisplayName == "" {
		data.DisplayName = "Steve"
	}
}

// splitPong splits the pong data passed by ;, taking into account escaping these.
func splitPong(s string) []string {
	var runes []rune
	var tokens []string
	inEscape := false
	for _, r := range s {
		switch {
		case r == '\\':
			inEscape = true
		case r == ';':
			tokens = append(tokens, string(runes))
			runes = runes[:0]
		case inEscape:
			inEscape = false
			fallthrough
		default:
			runes = append(runes, r)
		}
	}
	return append(tokens, string(runes))
}

// addressWithPongPort parses the redirect IPv4 port from the pong and returns the address passed with the port
// found if present, or the original address if not.
func addressWithPongPort(pong []byte, address string) string {
	frag := splitPong(string(pong))
	if len(frag) > 10 {
		portStr := frag[10]
		port, err := strconv.Atoi(portStr)
		// Vanilla (realms, in particular) will sometimes send port 19132 when you ping a port that isn't 19132 already,
		// but we should ignore that.
		if err != nil || port == 19132 {
			return address
		}
		// Remove the port from the address.
		addressParts := strings.Split(address, ":")
		address = strings.Join(strings.Split(address, ":")[:len(addressParts)-1], ":")
		return address + ":" + portStr
	}
	return address
}

// loadBotSkin 网易皮肤下载与处理（参考 bot-filler minecraft_neo skin 模块）。
// 普通皮肤返回 PNG；高级皮肤（如 4D）返回 ZIP 包，
// 内含皮肤 PNG（排除 _bloom.png）、geometry.json 与 manifest.json。
// 返回值均为 base64 编码，可直接写入 login.ClientData。
func loadBotSkin(url string, isSlim bool) (skinData, geometry, resourcePatch string, width, height int, skinUUID string, err error) {
	client := &http.Client{
		Timeout: 20 * time.Second,
		Transport: &http.Transport{
			DialContext: (&net.Dialer{
				Resolver: &net.Resolver{
					PreferGo: true,
					Dial: func(ctx context.Context, network, address string) (net.Conn, error) {
						return (&net.Dialer{}).DialContext(ctx, "udp", "8.8.8.8:53")
					},
				},
			}).DialContext,
		},
	}
	resp, err := client.Get(url)
	if err != nil {
		return "", "", "", 0, 0, "", fmt.Errorf("下载皮肤失败：%w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", "", "", 0, 0, "", fmt.Errorf("皮肤资源返回HTTP%d", resp.StatusCode)
	}
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 8*1024*1024+1))
	if len(raw) > 8*1024*1024 {
		return "", "", "", 0, 0, "", fmt.Errorf("皮肤资源超过8MiB")
	}
	if err != nil {
		return "", "", "", 0, 0, "", fmt.Errorf("读取皮肤失败：%w", err)
	}

	// 普通 PNG 皮肤：使用内置骨骼（64 空间 UV），resource patch 标识与内置几何保持一致。
	// 注意：统一使用宽臂几何 geometry.humanoid.custom —— 实测网易客户端对
	// geometry.humanoid.customSlim（细臂）条目不渲染（真实玩家条目均为宽臂）。
	geometryData := []byte(skinGeometry)
	resourcePatchData := bytes.Clone(skinResourcePatch)
	imageData := raw

	// 高级皮肤：ZIP 包
	if len(raw) >= 4 && bytes.Equal(raw[0:4], []byte("PK\x03\x04")) {
		var patch []byte
		imageData, geometryData, patch, skinUUID, err = loadSkinZIP(raw)
		if err != nil {
			return "", "", "", 0, 0, "", fmt.Errorf("处理皮肤 ZIP 失败：%w", err)
		}
		resourcePatchData = patch
	}

	config, err := png.DecodeConfig(bytes.NewReader(imageData))
	if err != nil || config.Width < 64 || config.Width > 1024 || config.Height < 16 || config.Height > 1024 {
		return "", "", "", 0, 0, "", fmt.Errorf("皮肤PNG尺寸无效")
	}
	img, err := png.Decode(bytes.NewReader(imageData))
	if err != nil {
		return "", "", "", 0, 0, "", fmt.Errorf("解码皮肤 PNG 失败：%w", err)
	}
	bounds := img.Bounds()
	w, h := bounds.Dx(), bounds.Dy()

	pixels := make([]byte, w*h*4)
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			r, g, b, a := img.At(x+bounds.Min.X, y+bounds.Min.Y).RGBA()
			idx := (y*w + x) * 4
			pixels[idx] = byte(r >> 8)
			pixels[idx+1] = byte(g >> 8)
			pixels[idx+2] = byte(b >> 8)
			pixels[idx+3] = byte(a >> 8)
		}
	}
	skinData = base64.StdEncoding.EncodeToString(pixels)
	geometry = base64.StdEncoding.EncodeToString(geometryData)
	resourcePatch = base64.StdEncoding.EncodeToString(resourcePatchData)
	return skinData, geometry, resourcePatch, w, h, skinUUID, nil
}

// downscaleSkin128To64 将 128x128 RGBA 皮肤像素 2x2 平均降采样为 64x64。
func downscaleSkin128To64(p []byte) []byte {
	out := make([]byte, 64*64*4)
	for y := 0; y < 64; y++ {
		for x := 0; x < 64; x++ {
			for c := 0; c < 4; c++ {
				sum := 0
				for dy := 0; dy < 2; dy++ {
					for dx := 0; dx < 2; dx++ {
						sum += int(p[((y*2+dy)*128+(x*2+dx))*4+c])
					}
				}
				out[(y*64+x)*4+c] = byte(sum / 4)
			}
		}
	}
	return out
}

// 网易高级皮肤 ZIP：皮肤 PNG（非 _bloom.png）、geometry.json、manifest.json（皮肤 UUID）。
// 无 geometry.json 时回退内置骨骼。
func loadSkinZIP(zipData []byte) (imageData, geometryData, resourcePatch []byte, skinUUID string, err error) {
	reader, err := zip.NewReader(bytes.NewReader(zipData), int64(len(zipData)))
	if err != nil {
		return nil, nil, nil, "", fmt.Errorf("打开皮肤 ZIP 失败：%w", err)
	}
	var geometryRaw []byte
	for _, file := range reader.File {
		switch {
		case strings.HasSuffix(file.Name, ".png") && !strings.HasSuffix(file.Name, "_bloom.png"):
			rc, err := file.Open()
			if err != nil {
				return nil, nil, nil, "", err
			}
			imageData, err = io.ReadAll(rc)
			_ = rc.Close()
			if err != nil {
				return nil, nil, nil, "", err
			}
		case strings.HasSuffix(file.Name, "geometry.json"):
			rc, err := file.Open()
			if err != nil {
				return nil, nil, nil, "", err
			}
			geometryRaw, err = io.ReadAll(rc)
			_ = rc.Close()
			if err != nil {
				return nil, nil, nil, "", err
			}
		case strings.HasSuffix(file.Name, "manifest.json"):
			rc, err := file.Open()
			if err != nil {
				return nil, nil, nil, "", err
			}
			manifestData, err := io.ReadAll(rc)
			_ = rc.Close()
			if err != nil {
				return nil, nil, nil, "", err
			}
			var manifest struct {
				Header struct {
					UUID string `json:"uuid"`
				} `json:"header"`
			}
			if json.Unmarshal(manifestData, &manifest) == nil {
				skinUUID = manifest.Header.UUID
			}
		}
	}
	if len(imageData) == 0 {
		return nil, nil, nil, "", fmt.Errorf("皮肤 ZIP 中未找到皮肤 PNG")
	}
	// 以 slim 标识为模板：processSkinGeometry 会替换为皮肤自带几何名
	resourcePatch = bytes.ReplaceAll(bytes.Clone(skinResourcePatch), []byte("geometry.humanoid.custom"), []byte("geometry.humanoid.customSlim"))
	if len(geometryRaw) > 0 {
		geometryData, resourcePatch, err = processSkinGeometry(geometryRaw, resourcePatch)
		if err != nil {
			return nil, nil, nil, "", fmt.Errorf("处理皮肤 ZIP 几何数据失败：%w", err)
		}
	} else {
		geometryData = []byte(skinGeometry)
	}
	return imageData, geometryData, resourcePatch, skinUUID, nil
}

// 网易高级皮肤 geometry.json 结构。
type skinGeometryCube struct {
	Inflate *json.Number  `json:"inflate,omitempty"`
	Mirror  *bool         `json:"mirror,omitempty"`
	Origin  []json.Number `json:"origin"`
	Size    []json.Number `json:"size"`
	Uv      []json.Number `json:"uv"`
}

// skinGeometryBone 描述皮肤几何中的一根骨骼。
type skinGeometryBone struct {
	Cubes         *[]skinGeometryCube `json:"cubes,omitempty"`
	Name          string              `json:"name"`
	Parent        string              `json:"parent,omitempty"`
	Pivot         []json.Number       `json:"pivot"`
	RenderGroupID int                 `json:"render_group_id,omitempty"`
	Rotation      []json.Number       `json:"rotation,omitempty"`
}

// skinGeometryData 描述皮肤几何数据。
type skinGeometryData struct {
	Bones               []*skinGeometryBone `json:"bones"`
	TextureHeight       int                 `json:"textureheight"`
	TextureWidth        int                 `json:"texturewidth"`
	VisibleBoundsHeight json.Number         `json:"visible_bounds_height,omitempty"`
	VisibleBoundsOffset []json.Number       `json:"visible_bounds_offset,omitempty"`
	VisibleBoundsWidth  json.Number         `json:"visible_bounds_width,omitempty"`
}

// processSkinGeometry 修复网易高级皮肤的 geometry.json：
// 修正骨骼父子关系、设置手臂渲染组、确保 root 骨骼存在，
// 并返回指向皮肤自身几何标识的 resource patch。
// 若 patch 中引用的几何标识不存在，客户端会拒绝该皮肤并回退默认皮肤。
func processSkinGeometry(rawData, patch []byte) (geometryJSON, resourcePatch []byte, err error) {
	geometryMap := map[string]json.RawMessage{}
	if err = json.Unmarshal(rawData, &geometryMap); err != nil {
		return nil, nil, fmt.Errorf("ProcessGeometry: %w", err)
	}
	var skinGeometryRaw json.RawMessage
	var geometryName string
	for k, v := range geometryMap {
		if strings.HasPrefix(k, "geometry.") {
			geometryName = k
			skinGeometryRaw = v
			break
		}
	}
	if geometryName == "" {
		return nil, nil, fmt.Errorf("处理皮肤几何数据失败：缺少几何数据")
	}
	resourcePatch = bytes.ReplaceAll(patch, []byte("geometry.humanoid.customSlim"), []byte(geometryName))

	geometry := &skinGeometryData{}
	if err = json.Unmarshal(skinGeometryRaw, geometry); err != nil {
		return nil, nil, fmt.Errorf("ProcessGeometry: %w", err)
	}
	// 修正骨骼父子关系并设置渲染组
	hasRoot := false
	renderGroupChildren := []string{"leftArm", "rightArm"}
	for _, bone := range geometry.Bones {
		switch bone.Name {
		case "waist", "leftLeg", "rightLeg":
			bone.Parent = "root"
		case "head":
			bone.Parent = "body"
		case "leftArm", "rightArm":
			bone.Parent = "body"
			bone.RenderGroupID = 1
		case "body":
			bone.Parent = "waist"
		case "root":
			hasRoot = true
		}
		if slices.Contains(renderGroupChildren, bone.Parent) {
			bone.RenderGroupID = 1
			renderGroupChildren = append(renderGroupChildren, bone.Name)
		}
	}
	if !hasRoot {
		geometry.Bones = append(geometry.Bones, &skinGeometryBone{
			Name:  "root",
			Pivot: []json.Number{json.Number("0.0"), json.Number("0.0"), json.Number("0.0")},
		})
	}
	geometryJSON, _ = json.Marshal(map[string]any{
		"format_version": "1.8.0",
		geometryName:     geometry,
	})
	return geometryJSON, resourcePatch, nil
}

// generateDefaultSkinRGBA 生成纯黑 64x32 默认皮肤（与 NexusEgo 对齐）
func generateDefaultSkinRGBA() []byte {
	w, h := 64, 32
	pixels := make([]byte, w*h*4)
	for i := 0; i < len(pixels); i += 4 {
		pixels[i] = 0     // R
		pixels[i+1] = 0   // G
		pixels[i+2] = 0   // B
		pixels[i+3] = 255 // A
	}
	return pixels
}
