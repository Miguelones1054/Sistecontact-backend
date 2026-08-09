package fireapp

import (
	"context"
	"fmt"
	"os"

	firebase "firebase.google.com/go/v4"
	"firebase.google.com/go/v4/auth"
	"cloud.google.com/go/firestore"
	"google.golang.org/api/option"
)

type App struct {
	Auth      *auth.Client
	Firestore *firestore.Client
}

// New inicializa Firebase Admin (Auth + Firestore) con la service account.
func New(ctx context.Context, credentialsFile string) (*App, error) {
	if credentialsFile == "" {
		return nil, fmt.Errorf("FIREBASE_CREDENTIALS_FILE es obligatorio")
	}
	if _, err := os.Stat(credentialsFile); err != nil {
		return nil, fmt.Errorf("archivo de credenciales Firebase: %w", err)
	}

	opt := option.WithCredentialsFile(credentialsFile)
	app, err := firebase.NewApp(ctx, nil, opt)
	if err != nil {
		return nil, fmt.Errorf("firebase.NewApp: %w", err)
	}

	authClient, err := app.Auth(ctx)
	if err != nil {
		return nil, fmt.Errorf("firebase.Auth: %w", err)
	}

	fs, err := app.Firestore(ctx)
	if err != nil {
		return nil, fmt.Errorf("firebase.Firestore: %w", err)
	}

	return &App{Auth: authClient, Firestore: fs}, nil
}

func (a *App) Close() error {
	if a.Firestore != nil {
		return a.Firestore.Close()
	}
	return nil
}
