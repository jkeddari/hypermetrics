package ctxkeys

import (
	"context"

	"github.com/jkeddari/hypermetrics/internal/config"
	"github.com/jkeddari/hypermetrics/internal/model"
)

// contextKey is a type for context keys to avoid collisions
type contextKey string

const (
	UserKey         contextKey = "user"
	ProfileKey      contextKey = "profile"
	SubscriptionKey contextKey = "subscription"
	URLPathKey      contextKey = "url_path"
	ConfigKey       contextKey = "config"
	CSRFTokenKey    contextKey = "csrf_token"
)

func User(ctx context.Context) *model.User {
	user, _ := ctx.Value(UserKey).(*model.User)
	return user
}

func URLPath(ctx context.Context) string {
	path, _ := ctx.Value(URLPathKey).(string)
	return path
}

func WithUser(ctx context.Context, user *model.User) context.Context {
	return context.WithValue(ctx, UserKey, user)
}

func WithURLPath(ctx context.Context, path string) context.Context {
	return context.WithValue(ctx, URLPathKey, path)
}

func Config(ctx context.Context) *config.Config {
	cfg, _ := ctx.Value(ConfigKey).(*config.Config)
	return cfg
}

func WithConfig(ctx context.Context, cfg *config.Config) context.Context {
	return context.WithValue(ctx, ConfigKey, cfg)
}

func Profile(ctx context.Context) *model.Profile {
	profile, _ := ctx.Value(ProfileKey).(*model.Profile)
	return profile
}

func WithProfile(ctx context.Context, profile *model.Profile) context.Context {
	return context.WithValue(ctx, ProfileKey, profile)
}

func Subscription(ctx context.Context) *model.Subscription {
	subscription, _ := ctx.Value(SubscriptionKey).(*model.Subscription)
	return subscription
}

func WithSubscription(ctx context.Context, subscription *model.Subscription) context.Context {
	return context.WithValue(ctx, SubscriptionKey, subscription)
}

func CSRFToken(ctx context.Context) string {
	token, _ := ctx.Value(CSRFTokenKey).(string)
	return token
}

func WithCSRFToken(ctx context.Context, token string) context.Context {
	return context.WithValue(ctx, CSRFTokenKey, token)
}
