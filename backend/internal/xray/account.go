package xray

import (
	"errors"
	"fmt"
	"strings"

	"github.com/xtls/xray-core/common/protocol"
	"github.com/xtls/xray-core/common/serial"
	"github.com/xtls/xray-core/proxy/shadowsocks"
	"github.com/xtls/xray-core/proxy/trojan"
	"github.com/xtls/xray-core/proxy/vless"
	"github.com/xtls/xray-core/proxy/vmess"

	"google.golang.org/protobuf/proto"
)

// NodeView 是 Account 构造所需的节点视图，由实例检测归一化产物与推送目标复用，
// 避免 xray 包反向依赖 node 包形成环。
type NodeView struct {
	Protocol string
	Cipher   string
	Flow     string
	Tag      string
	Host     string
	Port     int
}

// UserEmail 返回 Xray 面板用户 email：user-{id}@vpn.local，全小写。
func UserEmail(id int64) string {
	return fmt.Sprintf("user-%d@vpn.local", id)
}

// BuildUser 根据节点协议构造 Xray AddUser 所需 User。
func BuildUser(userID int64, uuid, proxySecret string, node NodeView) (*protocol.User, error) {
	account, err := accountOf(uuid, proxySecret, node)
	if err != nil {
		return nil, err
	}
	return &protocol.User{
		Level:   0,
		Email:   UserEmail(userID),
		Account: serial.ToTypedMessage(account),
	}, nil
}

// AccountFromNode 返回指定协议的 TypedMessage（供测试/扩展使用）。
func AccountFromNode(protocolName string, uuid, proxySecret string, node NodeView) (*serial.TypedMessage, error) {
	account, err := accountOf(uuid, proxySecret, node)
	if err != nil {
		return nil, err
	}
	return serial.ToTypedMessage(account), nil
}

func accountOf(uuid, proxySecret string, node NodeView) (proto.Message, error) {
	switch node.Protocol {
	case "vless":
		return &vless.Account{
			Id:         strings.ToLower(uuid),
			Flow:       node.Flow,
			Encryption: "none",
		}, nil
	case "vmess":
		return &vmess.Account{Id: uuid}, nil // 新版 Xray 已移除 alter_id，不要设置
	case "trojan":
		return &trojan.Account{Password: proxySecret}, nil
	case "shadowsocks", "ss":
		cipherType, err := cipherTypeOf(node.Cipher)
		if err != nil {
			return nil, err
		}
		return &shadowsocks.Account{Password: proxySecret, CipherType: cipherType}, nil
	default:
		return nil, fmt.Errorf("不支持的 Xray 节点协议: %s", node.Protocol)
	}
}

// cipherTypeOf 将节点 protocol_json 中记录的 cipher 字符串映射为 shadowsocks.CipherType。
func cipherTypeOf(cipher string) (shadowsocks.CipherType, error) {
	switch strings.ToLower(strings.TrimSpace(cipher)) {
	case "", "auto", "none":
		return shadowsocks.CipherType_NONE, nil
	case "chacha20-ietf-poly1305", "chacha20-poly1305":
		return shadowsocks.CipherType_CHACHA20_POLY1305, nil
	case "xchacha20-ietf-poly1305", "xchacha20-poly1305":
		return shadowsocks.CipherType_XCHACHA20_POLY1305, nil
	case "aes-256-gcm":
		return shadowsocks.CipherType_AES_256_GCM, nil
	case "aes-128-gcm":
		return shadowsocks.CipherType_AES_128_GCM, nil
	default:
		return shadowsocks.CipherType_UNKNOWN, fmt.Errorf("未知 shadowsocks cipher: %s", cipher)
	}
}

// cipherNameOf 将 shadowsocks.CipherType 反查为 protocol_json 使用的规范化字符串。
func cipherNameOf(t shadowsocks.CipherType) string {
	switch t {
	case shadowsocks.CipherType_CHACHA20_POLY1305:
		return "chacha20-ietf-poly1305"
	case shadowsocks.CipherType_XCHACHA20_POLY1305:
		return "xchacha20-ietf-poly1305"
	case shadowsocks.CipherType_AES_256_GCM:
		return "aes-256-gcm"
	case shadowsocks.CipherType_AES_128_GCM:
		return "aes-128-gcm"
	default:
		return ""
	}
}

// ErrUnsupportedProtocol 供调用方识别不支持协议。
var ErrUnsupportedProtocol = errors.New("不支持的 Xray 节点协议")
