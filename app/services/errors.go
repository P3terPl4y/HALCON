package services

import "errors"

var (
	ErrDriverNotFound      = errors.New("driver no encontrado")
	ErrClientNotFound      = errors.New("client no encontrado")
	ErrDriverAlreadyExists = errors.New("el usuario ya tiene un perfil de driver")
	ErrClientAlreadyExists = errors.New("el usuario ya tiene un perfil de client")
)
