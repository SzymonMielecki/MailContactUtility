package database

import (
	"context"
	"time"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

type DatabaseConfig struct {
	Host     string
	User     string
	Password string
	Database string
}

type Database struct {
	db *gorm.DB
}

type Token struct {
	*gorm.Model
	Email          string
	AccessToken    string
	TokenType      string
	RefreshToken   string
	Expiry         time.Time
	Scopes_as_json string
}

func NewDatabase(ctx context.Context, config DatabaseConfig) (*Database, error) {
	db, err := gorm.Open(postgres.Open(
		"host="+config.Host+" user="+config.User+" dbname="+config.Database+" password="+config.Password+" port=5432 sslmode=disable",
	), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent)})

	if err != nil {
		return nil, err
	}

	db.AutoMigrate(&Token{})
	return &Database{db: db}, nil
}

func (d *Database) AddToken(token Token) error {
	return d.db.Create(&token).Error
}
func (d *Database) GetTokens() ([]Token, error) {
	var tokens []Token
	if err := d.db.Find(&tokens).Error; err != nil {
		return nil, err
	}
	return tokens, nil
}

func (d *Database) GetTokensForScopes(scopes_as_json string) ([]Token, error) {
	var tokens []Token
	if err := d.db.Where("scopes_as_json = ?", scopes_as_json).Find(&tokens).Error; err != nil {
		return nil, err
	}
	return tokens, nil
}

func (d *Database) GetToken(email, scopes_as_json string) (Token, error) {
	var token Token
	if err := d.db.Where("email = ?", email).Where("scopes_as_json = ?", scopes_as_json).First(&token).Error; err != nil {
		return Token{}, err
	}
	return token, nil
}

func (d *Database) CheckExistsToken(email, scopes_as_json string) (bool, error) {
	var token Token
	if err := d.db.Where("email = ?", email).Where("scopes_as_json = ?", scopes_as_json).First(&token).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return false, nil
		}
		return false, err
	}
	return true, nil
}
