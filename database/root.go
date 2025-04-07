package database

import (
	"context"
	"time"

	"golang.org/x/oauth2"
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

func (d *Database) UpdateToken(email string, token *oauth2.Token) error {
	return d.db.Model(&Token{}).Where("email = ?", email).Updates(Token{
		AccessToken:  token.AccessToken,
		TokenType:    token.TokenType,
		RefreshToken: token.RefreshToken,
		Expiry:       token.Expiry,
	}).Error
}

type Token struct {
	Email        string
	AccessToken  string
	TokenType    string
	RefreshToken string
	Expiry       time.Time
}

func NewDatabase(ctx context.Context, config DatabaseConfig) (*Database, error) {
	db, err := gorm.Open(postgres.Open(
		"host="+config.Host+" user="+config.User+" password="+config.Password+" dbname="+config.Database+" port=5432 sslmode=disable",
	), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		return nil, err
	}
	db.AutoMigrate(&Token{})
	return &Database{db: db}, nil
}

func (d *Database) AddToken(token Token) error {
	return d.db.Model(&Token{}).Create(&token).Error
}

func (d *Database) GetEmails() ([]string, error) {
	var emails []string
	if err := d.db.Model(&Token{}).Pluck("email", &emails).Error; err != nil {
		return nil, err
	}
	return emails, nil
}

func (d *Database) GetToken(email string) (*Token, error) {
	var token Token
	if err := d.db.Model(&Token{}).Where("email = ?", email).First(&token).Error; err != nil {
		return nil, err
	}
	return &token, nil
}
