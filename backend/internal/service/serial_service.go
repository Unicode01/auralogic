package service

import (
	"fmt"
	"math/rand"
	"strings"
	"time"

	"auralogic/internal/models"
	"auralogic/internal/repository"
)

type SerialService struct {
	serialRepo  *repository.SerialRepository
	productRepo *repository.ProductRepository
	orderRepo   *repository.OrderRepository
}

func NewSerialService(
	serialRepo *repository.SerialRepository,
	productRepo *repository.ProductRepository,
	orderRepo *repository.OrderRepository,
) *SerialService {
	return &SerialService{
		serialRepo:  serialRepo,
		productRepo: productRepo,
		orderRepo:   orderRepo,
	}
}

// GenerateAntiCounterfeitCode 生成4位防伪码 (0-9 A-Z)
func (s *SerialService) GenerateAntiCounterfeitCode() string {
	const charset = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZ"
	rand.Seed(time.Now().UnixNano())

	code := make([]byte, 4)
	for i := range code {
		code[i] = charset[rand.Intn(len(charset))]
	}
	return string(code)
}

// FormatSequenceNumber 格式化序号 (001, 002, ..., 1000, ...)
func (s *SerialService) FormatSequenceNumber(seq int) string {
	if seq < 1000 {
		return fmt.Sprintf("%03d", seq)
	} else if seq < 10000 {
		return fmt.Sprintf("%04d", seq)
	} else if seq < 100000 {
		return fmt.Sprintf("%05d", seq)
	}
	return fmt.Sprintf("%d", seq)
}

// GenerateSerialNumber 生成完整序列号: 产品码+序号+防伪码
func (s *SerialService) GenerateSerialNumber(productCode string, sequenceNumber int, antiCounterfeitCode string) string {
	seqStr := s.FormatSequenceNumber(sequenceNumber)
	return fmt.Sprintf("%s%s%s", productCode, seqStr, antiCounterfeitCode)
}

// CreateSerialForOrder 为订单商品创建序列号
func (s *SerialService) CreateSerialForOrder(orderID uint, productID uint, quantity int) ([]models.ProductSerial, error) {
	// 获取商品信息
	product, err := s.productRepo.FindByID(productID)
	if err != nil {
		return nil, fmt.Errorf("product not found: %w", err)
	}

	// 检查商品是否设置了产品码
	if product.ProductCode == "" {
		return nil, fmt.Errorf("product code not set for product %d", productID)
	}

	// 获取订单信息（验证订单存在）
	_, err = s.orderRepo.FindByID(orderID)
	if err != nil {
		return nil, fmt.Errorf("order not found: %w", err)
	}

	// 生成序列号
	serials := make([]models.ProductSerial, 0, quantity)
	for i := 0; i < quantity; i++ {
		// 获取下一个序号
		nextSeq, err := s.serialRepo.GetNextSequenceNumber(productID)
		if err != nil {
			return nil, fmt.Errorf("failed to get next sequence number: %w", err)
		}

		// 生成防伪码
		antiCounterfeitCode := s.GenerateAntiCounterfeitCode()

		// 生成完整序列号
		serialNumber := s.GenerateSerialNumber(product.ProductCode, nextSeq, antiCounterfeitCode)

		serial := models.ProductSerial{
			SerialNumber:        serialNumber,
			ProductID:           productID,
			OrderID:             orderID,
			ProductCode:         product.ProductCode,
			SequenceNumber:      nextSeq,
			AntiCounterfeitCode: antiCounterfeitCode,
			ViewCount:           0,
		}

		serials = append(serials, serial)
	}

	// 批量创建
	if err := s.serialRepo.BatchCreate(serials); err != nil {
		return nil, fmt.Errorf("failed to create serials: %w", err)
	}

	return serials, nil
}

// VerifySerial 验证序列号并增加查看次数
func (s *SerialService) VerifySerial(serialNumber string) (*models.ProductSerial, error) {
	// 查找序列号
	serial, err := s.serialRepo.FindBySerialNumber(strings.ToUpper(serialNumber))
	if err != nil {
		return nil, fmt.Errorf("serial number not found")
	}

	// 增加查看次数
	if err := s.serialRepo.IncrementViewCount(serial.SerialNumber); err != nil {
		// 记录错误但不影响返回结果
	}

	// 重新加载以获取最新的查看次数
	serial, _ = s.serialRepo.FindBySerialNumber(serial.SerialNumber)

	return serial, nil
}

// GetSerialsByOrderID 获取订单的所有序列号
func (s *SerialService) GetSerialsByOrderID(orderID uint) ([]models.ProductSerial, error) {
	return s.serialRepo.FindByOrderID(orderID)
}

// GetSerialsByProductID 获取商品的所有序列号
func (s *SerialService) GetSerialsByProductID(productID uint) ([]models.ProductSerial, error) {
	return s.serialRepo.FindByProductID(productID)
}

// ListSerials 分页查询序列号
func (s *SerialService) ListSerials(page, limit int, filters map[string]interface{}) ([]models.ProductSerial, int64, error) {
	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > 100 {
		limit = 20
	}

	return s.serialRepo.List(page, limit, filters)
}

// GetStatistics 获取统计信息
func (s *SerialService) GetStatistics() (map[string]interface{}, error) {
	return s.serialRepo.GetStatistics()
}

// DeleteSerial Delete a single serial number by ID
func (s *SerialService) DeleteSerial(id uint) error {
	return s.serialRepo.Delete(id)
}

// DeleteSerialsByOrderID Delete all serial numbers for an order
func (s *SerialService) DeleteSerialsByOrderID(orderID uint) error {
	return s.serialRepo.DeleteByOrderID(orderID)
}

// BatchDeleteSerials Delete multiple serial numbers by IDs
func (s *SerialService) BatchDeleteSerials(ids []uint) error {
	return s.serialRepo.BatchDelete(ids)
}
