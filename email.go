package main

import (
	"log"
	"sync"
	"time"
)

type EmailSender interface {
	SendCode(email, code string) error
}

type DefaultEmailSender struct{}

func (d *DefaultEmailSender) SendCode(email, code string) error {
	log.Printf("[EmailService] Sending verification code %s to %s", code, email)
	return nil
}

var CurrentEmailSender EmailSender = &DefaultEmailSender{}

type codeRecord struct {
	code      string
	expiresAt time.Time
}

var (
	codeMu    sync.RWMutex
	codeStore = make(map[string]codeRecord)
)

func StoreCode(email, code string) {
	codeMu.Lock()
	defer codeMu.Unlock()
	codeStore[email] = codeRecord{
		code:      code,
		expiresAt: time.Now().Add(10 * time.Minute),
	}
}

func VerifyCode(email, code string) bool {
	codeMu.Lock()
	defer codeMu.Unlock()

	record, exists := codeStore[email]
	if !exists {
		return false
	}

	if time.Now().After(record.expiresAt) {
		delete(codeStore, email)
		return false
	}

	if record.code != code {
		return false
	}
	delete(codeStore, email)
	return true
}

func ClearCodeStore() {
	codeMu.Lock()
	defer codeMu.Unlock()
	codeStore = make(map[string]codeRecord)
}
