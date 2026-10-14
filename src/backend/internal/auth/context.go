package auth

import "context"

type contextKey string

const authenticatedUserKey contextKey = "authenticated_user"

func WithUser(
	ctx context.Context,
	user *User,
) context.Context {
	return context.WithValue(
		ctx,
		authenticatedUserKey,
		user,
	)
}

func UserFromContext(
	ctx context.Context,
) (*User, bool) {
	user, ok := ctx.Value(authenticatedUserKey).(*User)
	return user, ok
}
