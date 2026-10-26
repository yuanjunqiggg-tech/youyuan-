package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

// 机器人人格卡 / 反 AI 腔 / 群内风格学习（设计 docs/design/bot-persona.md）。
//
// 目标：群里表现像「管事的老群友」，不是客服，也不是朗读提示词的模型。
// 本文件只做三件事：人格卡持久化（qq-persona.json）、反 AI 腔提示词块、
// 群风格统计档案（qq-style.json）。群风格只存统计量（频率/长度/时段），
// 不存发送者身份；提示词块是纯文本，供主控注入 youyuan_qq_ai.go 的 system prompt。

// yyQQPersona 人格卡：机器人以什么身份、什么语气在群里说话。
type yyQQPersona struct {
	Name    string   `json:"name"`    // 群里自称，如「老白」
	Persona string   `json:"persona"` // 人设描述
	Style   string   `json:"style"`   // 语言风格
	Taboos  []string `json:"taboos"`  // 禁忌
	AntiAI  []string `json:"anti_ai"` // 反 AI 腔规则
	Enabled bool     `json:"enabled"` // 总开关
}

// yyStyleConfig 群风格学习配置，可整体关闭。
type yyStyleConfig struct {
	Enabled bool `json:"enabled"`
	MinMsgs int  `json:"min_msgs"` // 少于此样本量不出风格摘要
	TopN    int  `json:"top_n"`    // 高频词取前 N 个
}

// yyStyleSample 群风格环形缓冲的一条：只留文本与时间，不存发送者。
type yyStyleSample struct {
	Text string    `json:"t"`
	At   time.Time `json:"at"`
}

// yyStyleWord 一个高频词及其出现次数。
type yyStyleWord struct {
	Text string `json:"t"`
	N    int    `json:"n"`
}

// yyStyleStat 一个群的风格统计结果。
type yyStyleStat struct {
	Group     string        `json:"group"`
	Samples   int           `json:"samples"`
	AvgRunes  float64       `json:"avg_runes"`
	FaceRatio float64       `json:"face_ratio"` // 带表情/图片标记的消息占比
	TopWords  []yyStyleWord `json:"top_words"`
	Hours     [24]int       `json:"hours"`      // 活跃时段直方图
	PeakHours [2]int        `json:"peak_hours"` // 最活跃时段起止（含）
}

const (
	yyQQPersonaBlockMax = 800 // 提示词块总长上限（字）
	yyQQStyleBufferMax  = 200 // 每群环形缓冲条数
	yyQQStyleMaxGroups  = 32  // 全局群上限（与 yyQQMemory 一致）
	yyQQStyleSampleClip = 500 // 单条样本截断（字）
	yyQQPersonaSummary  = 120 // 风格摘要上限（字）
)

// yyQQPersonaState 人格卡与风格配置的持久化状态，范式照 youyuan_qq_memory.go：
// 启动加载、变更即存、JSON 损坏回退默认并记日志。
var yyQQPersonaState = struct {
	sync.Mutex
	Path      string
	Persona   yyQQPersona
	Style     yyStyleConfig
	DiskError string
}{Persona: yyQQPersonaDefault(), Style: yyStyleConfigDefault()}

// yyQQStyleState 群风格样本环形缓冲，落盘 qq-style.json。
var yyQQStyleState = struct {
	sync.Mutex
	Path      string
	Groups    map[string][]yyStyleSample
	DiskError string
}{Groups: map[string][]yyStyleSample{}}
