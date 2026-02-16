package service

import (
	"bytes"
	crand "crypto/rand"
	"encoding/base64"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"math"
	"math/big"
	"math/rand"
	"net/http"
	"net/url"
	"strings"
	"time"

	"auralogic/internal/config"
	"auralogic/internal/pkg/cache"
)

// CaptchaService 验证码服务
type CaptchaService struct{}

func NewCaptchaService() *CaptchaService {
	return &CaptchaService{}
}

func init() {
	// Avoid deterministic captcha noise patterns on process start.
	var b [8]byte
	if _, err := crand.Read(b[:]); err == nil {
		rand.Seed(int64(binary.LittleEndian.Uint64(b[:])))
	} else {
		rand.Seed(time.Now().UnixNano())
	}
}

// VerifyCaptcha 验证验证码token
func (s *CaptchaService) VerifyCaptcha(token string, remoteIP string) error {
	cfg := config.GetConfig()
	captchaCfg := cfg.Security.Captcha

	switch captchaCfg.Provider {
	case "none", "":
		return nil
	case "cloudflare":
		return s.verifyCloudflare(captchaCfg.SecretKey, token, remoteIP)
	case "google":
		return s.verifyGoogle(captchaCfg.SecretKey, token, remoteIP)
	case "builtin":
		return s.verifyBuiltin(token)
	default:
		return fmt.Errorf("unknown captcha provider: %s", captchaCfg.Provider)
	}
}

// NeedCaptcha 判断指定场景是否需要验证码
func (s *CaptchaService) NeedCaptcha(scene string) bool {
	cfg := config.GetConfig()
	captchaCfg := cfg.Security.Captcha

	if captchaCfg.Provider == "none" || captchaCfg.Provider == "" {
		return false
	}

	switch scene {
	case "login":
		return captchaCfg.EnableForLogin
	case "register":
		return captchaCfg.EnableForRegister
	case "serial_verify":
		return captchaCfg.EnableForSerialVerify
	default:
		return false
	}
}

// GenerateBuiltinCaptcha 生成内置验证码，返回captcha ID和PNG base64
func (s *CaptchaService) GenerateBuiltinCaptcha() (string, string, error) {
	// Use crypto-rand to prevent predictability (the image noise uses math/rand).
	codeN, err := crand.Int(crand.Reader, big.NewInt(10000))
	if err != nil {
		return "", "", fmt.Errorf("failed to generate captcha code: %w", err)
	}
	code := fmt.Sprintf("%04d", codeN.Int64())

	idBytes := make([]byte, 16)
	if _, err := crand.Read(idBytes); err != nil {
		return "", "", fmt.Errorf("failed to generate captcha id: %w", err)
	}
	captchaID := "captcha:" + hex.EncodeToString(idBytes)

	err = cache.Set(captchaID, code, 5*time.Minute)
	if err != nil {
		return "", "", fmt.Errorf("failed to store captcha: %w", err)
	}

	imgBase64, err := generateCaptchaPNG(code)
	if err != nil {
		return "", "", fmt.Errorf("failed to generate captcha image: %w", err)
	}

	return captchaID, imgBase64, nil
}

// verifyBuiltin 验证内置验证码
func (s *CaptchaService) verifyBuiltin(token string) error {
	var captchaID, userCode string
	for i := len(token) - 1; i >= 0; i-- {
		if token[i] == ':' {
			captchaID = token[:i]
			userCode = token[i+1:]
			break
		}
	}
	if captchaID == "" || userCode == "" {
		return fmt.Errorf("invalid captcha token format")
	}

	// Prevent using this endpoint as an oracle/deleter for arbitrary Redis keys.
	if !strings.HasPrefix(captchaID, "captcha:") {
		return fmt.Errorf("invalid captcha token format")
	}
	idPart := strings.TrimPrefix(captchaID, "captcha:")
	if len(idPart) != 32 {
		return fmt.Errorf("invalid captcha token format")
	}
	if _, err := hex.DecodeString(idPart); err != nil {
		return fmt.Errorf("invalid captcha token format")
	}

	storedCode, err := cache.Get(captchaID)
	if err != nil {
		return fmt.Errorf("captcha expired or invalid")
	}

	_ = cache.Del(captchaID) // one-time use best-effort

	if storedCode != userCode {
		return fmt.Errorf("incorrect captcha code")
	}

	return nil
}

// verifyCloudflare 验证Cloudflare Turnstile
func (s *CaptchaService) verifyCloudflare(secret, token, remoteIP string) error {
	resp, err := http.PostForm("https://challenges.cloudflare.com/turnstile/v0/siteverify", url.Values{
		"secret":   {secret},
		"response": {token},
		"remoteip": {remoteIP},
	})
	if err != nil {
		return fmt.Errorf("failed to verify captcha: %w", err)
	}
	defer resp.Body.Close()

	var result struct {
		Success bool `json:"success"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return fmt.Errorf("failed to parse captcha response: %w", err)
	}

	if !result.Success {
		return fmt.Errorf("captcha verification failed")
	}

	return nil
}

// verifyGoogle 验证Google reCAPTCHA
func (s *CaptchaService) verifyGoogle(secret, token, remoteIP string) error {
	resp, err := http.PostForm("https://www.google.com/recaptcha/api/siteverify", url.Values{
		"secret":   {secret},
		"response": {token},
		"remoteip": {remoteIP},
	})
	if err != nil {
		return fmt.Errorf("failed to verify captcha: %w", err)
	}
	defer resp.Body.Close()

	var result struct {
		Success bool `json:"success"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return fmt.Errorf("failed to parse captcha response: %w", err)
	}

	if !result.Success {
		return fmt.Errorf("captcha verification failed")
	}

	return nil
}

// ==================== PNG 验证码生成 ====================

const (
	captchaW = 160
	captchaH = 50
)

// 5x7 点阵字体，每个数字用 5 列 x 7 行的 bool 矩阵表示
var digitFont = map[byte][7][5]bool{
	'0': {
		{false, true, true, true, false},
		{true, false, false, false, true},
		{true, false, false, true, true},
		{true, false, true, false, true},
		{true, true, false, false, true},
		{true, false, false, false, true},
		{false, true, true, true, false},
	},
	'1': {
		{false, false, true, false, false},
		{false, true, true, false, false},
		{true, false, true, false, false},
		{false, false, true, false, false},
		{false, false, true, false, false},
		{false, false, true, false, false},
		{true, true, true, true, true},
	},
	'2': {
		{false, true, true, true, false},
		{true, false, false, false, true},
		{false, false, false, false, true},
		{false, false, false, true, false},
		{false, false, true, false, false},
		{false, true, false, false, false},
		{true, true, true, true, true},
	},
	'3': {
		{false, true, true, true, false},
		{true, false, false, false, true},
		{false, false, false, false, true},
		{false, false, true, true, false},
		{false, false, false, false, true},
		{true, false, false, false, true},
		{false, true, true, true, false},
	},
	'4': {
		{false, false, false, true, false},
		{false, false, true, true, false},
		{false, true, false, true, false},
		{true, false, false, true, false},
		{true, true, true, true, true},
		{false, false, false, true, false},
		{false, false, false, true, false},
	},
	'5': {
		{true, true, true, true, true},
		{true, false, false, false, false},
		{true, true, true, true, false},
		{false, false, false, false, true},
		{false, false, false, false, true},
		{true, false, false, false, true},
		{false, true, true, true, false},
	},
	'6': {
		{false, true, true, true, false},
		{true, false, false, false, false},
		{true, false, false, false, false},
		{true, true, true, true, false},
		{true, false, false, false, true},
		{true, false, false, false, true},
		{false, true, true, true, false},
	},
	'7': {
		{true, true, true, true, true},
		{false, false, false, false, true},
		{false, false, false, true, false},
		{false, false, true, false, false},
		{false, false, true, false, false},
		{false, false, true, false, false},
		{false, false, true, false, false},
	},
	'8': {
		{false, true, true, true, false},
		{true, false, false, false, true},
		{true, false, false, false, true},
		{false, true, true, true, false},
		{true, false, false, false, true},
		{true, false, false, false, true},
		{false, true, true, true, false},
	},
	'9': {
		{false, true, true, true, false},
		{true, false, false, false, true},
		{true, false, false, false, true},
		{false, true, true, true, true},
		{false, false, false, false, true},
		{false, false, false, false, true},
		{false, true, true, true, false},
	},
}

// generateCaptchaPNG 生成 PNG 格式验证码并返回 base64 编码
func generateCaptchaPNG(code string) (string, error) {
	img := image.NewRGBA(image.Rect(0, 0, captchaW, captchaH))

	// 随机浅色背景
	bgR := uint8(230 + rand.Intn(25))
	bgG := uint8(230 + rand.Intn(25))
	bgB := uint8(230 + rand.Intn(25))
	bg := color.RGBA{bgR, bgG, bgB, 255}
	for y := 0; y < captchaH; y++ {
		for x := 0; x < captchaW; x++ {
			img.Set(x, y, bg)
		}
	}

	// 干扰：随机噪点
	for i := 0; i < 200; i++ {
		x := rand.Intn(captchaW)
		y := rand.Intn(captchaH)
		c := randomDarkColor()
		c.A = uint8(40 + rand.Intn(60))
		img.Set(x, y, c)
	}

	// 干扰：曲线
	for i := 0; i < 4; i++ {
		drawBezierCurve(img, randomDarkColor(), 1+rand.Intn(2))
	}

	// 绘制字符
	pixelSize := 4 // 每个点阵像素放大为 4x4
	charW := 5*pixelSize + 6
	totalW := len(code) * charW
	startX := (captchaW - totalW) / 2

	for i := 0; i < len(code); i++ {
		ch := code[i]
		glyph, ok := digitFont[ch]
		if !ok {
			continue
		}

		cx := startX + i*charW + charW/2
		cy := captchaH / 2
		angle := (rand.Float64() - 0.5) * 0.5 // -0.25 ~ 0.25 弧度（约 -14 ~ 14 度）
		offsetY := rand.Intn(8) - 4
		c := randomDarkColor()

		drawRotatedGlyph(img, glyph, cx, cy+offsetY, pixelSize, angle, c)
	}

	// 干扰：覆盖线段
	for i := 0; i < 6; i++ {
		c := randomDarkColor()
		c.A = uint8(50 + rand.Intn(80))
		drawLine(img, rand.Intn(captchaW), rand.Intn(captchaH), rand.Intn(captchaW), rand.Intn(captchaH), c)
	}

	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		return "", err
	}

	return "data:image/png;base64," + base64.StdEncoding.EncodeToString(buf.Bytes()), nil
}

func randomDarkColor() color.RGBA {
	colors := []color.RGBA{
		{180, 40, 40, 255},
		{40, 100, 180, 255},
		{40, 160, 80, 255},
		{140, 60, 180, 255},
		{200, 120, 20, 255},
		{20, 140, 120, 255},
		{100, 80, 60, 255},
	}
	c := colors[rand.Intn(len(colors))]
	// 稍微随机化
	c.R = uint8(clamp(int(c.R)+rand.Intn(40)-20, 0, 255))
	c.G = uint8(clamp(int(c.G)+rand.Intn(40)-20, 0, 255))
	c.B = uint8(clamp(int(c.B)+rand.Intn(40)-20, 0, 255))
	return c
}

func clamp(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

// drawRotatedGlyph 将点阵字形绕中心旋转后绘制到图像上
func drawRotatedGlyph(img *image.RGBA, glyph [7][5]bool, cx, cy, pixelSize int, angle float64, c color.RGBA) {
	cosA := math.Cos(angle)
	sinA := math.Sin(angle)

	glyphW := 5 * pixelSize
	glyphH := 7 * pixelSize

	for row := 0; row < 7; row++ {
		for col := 0; col < 5; col++ {
			if !glyph[row][col] {
				continue
			}
			// 点阵像素的中心坐标（相对于字形中心）
			relX := float64(col*pixelSize+pixelSize/2) - float64(glyphW)/2
			relY := float64(row*pixelSize+pixelSize/2) - float64(glyphH)/2

			// 旋转
			rx := relX*cosA - relY*sinA
			ry := relX*sinA + relY*cosA

			// 绘制放大后的像素块
			for dy := -pixelSize / 2; dy < pixelSize/2; dy++ {
				for dx := -pixelSize / 2; dx < pixelSize/2; dx++ {
					px := cx + int(rx) + dx
					py := cy + int(ry) + dy
					if px >= 0 && px < captchaW && py >= 0 && py < captchaH {
						img.Set(px, py, c)
					}
				}
			}
		}
	}
}

// drawLine Bresenham 画线
func drawLine(img *image.RGBA, x0, y0, x1, y1 int, c color.RGBA) {
	dx := abs(x1 - x0)
	dy := abs(y1 - y0)
	sx := 1
	if x0 > x1 {
		sx = -1
	}
	sy := 1
	if y0 > y1 {
		sy = -1
	}
	err := dx - dy

	for {
		if x0 >= 0 && x0 < captchaW && y0 >= 0 && y0 < captchaH {
			blendPixel(img, x0, y0, c)
		}
		if x0 == x1 && y0 == y1 {
			break
		}
		e2 := 2 * err
		if e2 > -dy {
			err -= dy
			x0 += sx
		}
		if e2 < dx {
			err += dx
			y0 += sy
		}
	}
}

// drawBezierCurve 绘制三次贝塞尔干扰曲线
func drawBezierCurve(img *image.RGBA, c color.RGBA, thickness int) {
	x0 := float64(rand.Intn(20))
	y0 := float64(10 + rand.Intn(30))
	x1 := float64(30 + rand.Intn(50))
	y1 := float64(rand.Intn(captchaH))
	x2 := float64(80 + rand.Intn(50))
	y2 := float64(rand.Intn(captchaH))
	x3 := float64(captchaW - rand.Intn(20))
	y3 := float64(10 + rand.Intn(30))

	c.A = uint8(80 + rand.Intn(100))

	steps := 200
	for i := 0; i < steps; i++ {
		t := float64(i) / float64(steps)
		u := 1.0 - t
		px := u*u*u*x0 + 3*u*u*t*x1 + 3*u*t*t*x2 + t*t*t*x3
		py := u*u*u*y0 + 3*u*u*t*y1 + 3*u*t*t*y2 + t*t*t*y3

		ix := int(px)
		iy := int(py)
		for dy := -thickness / 2; dy <= thickness/2; dy++ {
			for dx := -thickness / 2; dx <= thickness/2; dx++ {
				nx := ix + dx
				ny := iy + dy
				if nx >= 0 && nx < captchaW && ny >= 0 && ny < captchaH {
					blendPixel(img, nx, ny, c)
				}
			}
		}
	}
}

// blendPixel 带 alpha 混合的像素绘制
func blendPixel(img *image.RGBA, x, y int, c color.RGBA) {
	existing := img.RGBAAt(x, y)
	alpha := float64(c.A) / 255.0
	inv := 1.0 - alpha
	img.Set(x, y, color.RGBA{
		R: uint8(float64(c.R)*alpha + float64(existing.R)*inv),
		G: uint8(float64(c.G)*alpha + float64(existing.G)*inv),
		B: uint8(float64(c.B)*alpha + float64(existing.B)*inv),
		A: 255,
	})
}

func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}
