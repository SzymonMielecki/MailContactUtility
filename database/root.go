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
	return d.db.Create(&token).Error
}

func (d *Database) GetEmails() ([]string, error) {
	var emails []string
	if err := d.db.Find(&emails).Error; err != nil {
		return nil, err
	}
	return emails, nil
}

func (d *Database) GetToken(email string) (Token, error) {
	var token Token
	if err := d.db.Where("email = ?", email).First(&token).Error; err != nil {
		return Token{}, err
	}
	return token, nil
}

package database

import (
	"context"
	"log"
	"time"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

type Token struct {
	*gorm.Model
	Email        string
	AccessToken  string
	RefreshToken string
	Expiry       time.Time
	TokenType    string
}

type DatabaseConfig struct {
	Host     string
	Password string
	User     string
	Name     string
}

type Database struct {
	*gorm.DB
}

func (d *Database) GetToken(email string) (Token, error) {
	var token Token
	result := d.Where("email = ?", email).First(&token)
	return token, result.Error
}

func (d *Database) AddToken(token Token) error {
	return d.Create(&token).Error
}

func NewDatabase(ctx context.Context, config DatabaseConfig) *Database {
	client, err := gorm.Open(postgres.Open("host="+config.Host+" user="+config.User+" password="+config.Password+" dbname="+config.Name+" port=5432 sslmode=disable timezone=UTC"), &gorm.Config{})
	if err != nil {
		log.Fatal(err)
	}
	client.AutoMigrate(&Token{})
	return &Database{client}
}

func (d *Database) GetEmails() ([]string, error) {
	var tokens []Token
	result := d.Find(&tokens)
	if result.Error != nil {
		return nil, result.Error
	}
	emails := make([]string, len(tokens))
	for i, token := range tokens {
		emails[i] = token.Email
	}
	return emails, nil
}
