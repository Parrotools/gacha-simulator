package auth

import (
	"crypto/ed25519"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"log"
	"os"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

var privateKey ed25519.PrivateKey
var publicKey ed25519.PublicKey

func init() {
	LoadOrGenerateKeys()
}

func LoadOrGenerateKeys() {
	privFile := "jwt_private.pem"
	pubFile := "jwt_public.pem"

	if _, err := os.Stat(privFile); os.IsNotExist(err) {
		if _, err := os.Stat("../" + privFile); err == nil {
			privFile = "../" + privFile
			pubFile = "../" + pubFile
		}
	}

	privPEM, errPriv := os.ReadFile(privFile)
	pubPEM, errPub := os.ReadFile(pubFile)

	if errPriv == nil && errPub == nil {
		blockPriv, _ := pem.Decode(privPEM)
		blockPub, _ := pem.Decode(pubPEM)
		if blockPriv != nil && blockPub != nil {
			parsedPriv, errParsePriv := x509.ParsePKCS8PrivateKey(blockPriv.Bytes)
			parsedPub, errParsePub := x509.ParsePKIXPublicKey(blockPub.Bytes)
			if errParsePriv == nil && errParsePub == nil {
				if privK, ok := parsedPriv.(ed25519.PrivateKey); ok {
					if pubK, ok2 := parsedPub.(ed25519.PublicKey); ok2 {
						privateKey = privK
						publicKey = pubK
						return
					}
				}
			}
		}
	}

	pub, priv, err := ed25519.GenerateKey(nil)
	if err != nil {
		log.Fatalf("initialise ED25519 failed:%v", err)
	}
	publicKey = pub
	privateKey = priv

	privBytes, err := x509.MarshalPKCS8PrivateKey(priv)
	if err == nil {
		block := &pem.Block{
			Type:  "PRIVATE KEY",
			Bytes: privBytes,
		}
		_ = os.WriteFile(privFile, pem.EncodeToMemory(block), 0600)
	}

	pubBytes, err := x509.MarshalPKIXPublicKey(pub)
	if err == nil {
		block := &pem.Block{
			Type:  "PUBLIC KEY",
			Bytes: pubBytes,
		}
		_ = os.WriteFile(pubFile, pem.EncodeToMemory(block), 0644)
	}
}

type CustomClaims struct {
	UserID string `json:"user_id"`
	Role   string `json:"role"`
	jwt.RegisteredClaims
}

func GenerateToken(userID string, role string) (string, error) {
	claims := CustomClaims{
		UserID: userID,
		Role:   role,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(24 * time.Hour)),
			Issuer:    "gacha-simulator",
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodEdDSA, claims)
	return token.SignedString(privateKey)
}

func ParseToken(tokenString string) (*CustomClaims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &CustomClaims{}, func(t *jwt.Token) (interface{}, error) {
		if _, ok := t.Method.(*jwt.SigningMethodEd25519); !ok {
			return nil, errors.New("signingMethod rejected")
		}
		return publicKey, nil
	})
	if err != nil {
		return nil, err
	}
	if claims, ok := token.Claims.(*CustomClaims); ok && token.Valid {
		return claims, nil
	}
	return nil, errors.New("token failed")
}
