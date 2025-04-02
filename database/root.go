package database

import (
	"context"
	"time"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
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
	Email        string
	AccessToken  string
	TokenType    string
	RefreshToken string
	Expiry       time.Time
}

func NewDatabase(ctx context.Context, config DatabaseConfig) (*Database, error) {
	db, err := gorm.Open(postgres.Open(
		"host="+config.Host+" user="+config.User+" password="+config.Password+" dbname="+config.Database+" port=5432 sslmode=disable",
	), &gorm.Config{})
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

func (d *Database) GetToken(email string) (Token, error) {
	var token Token
	if err := d.db.Model(&Token{}).Where("email = ?", email).First(&token).Error; err != nil {
		return Token{}, err
	}
	return token, nil
}
