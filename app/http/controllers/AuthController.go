package controllers

import (
	"strings"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/middleware/csrf"
	"github.com/gofiber/fiber/v3/middleware/session"
	"golang.org/x/crypto/bcrypt"
	"goravel/app/facades"
	"goravel/app/models"
	"goravel/app/requests"
	"goravel/app/services"
	"log"
)

type AuthController struct {
	userService *services.UserService
}

func NewAuthController() *AuthController {
	return &AuthController{
		userService: services.NewUserService(),
	}
}

// ShowRegister - GET /register
func (c *AuthController) ShowRegister(ctx fiber.Ctx) error {
	facades.Log().Debug("Mostrando formulario de registro")
	return ctx.Render("auth/register", fiber.Map{
		"title": "Crear cuenta", "csrfToken": csrf.TokenFromContext(ctx),
	})
}

// Register - POST /register
func (c *AuthController) Register(ctx fiber.Ctx) error {
	var req requests.CreateUserRequest
	if err := ctx.Bind().Body(&req); err != nil {
		facades.Log().Errorf("Error al bindear datos de registro: %v", err)
		return ctx.Render("auth/register", fiber.Map{
			"title":       "Crear cuenta",
			"flash_error": "Datos inválidos", "csrfToken": csrf.TokenFromContext(ctx),
		}, "layouts/base")
	}

	req.Role = "user"
	req.Email = strings.ToLower(strings.TrimSpace(req.Email))
	log.Printf("Intento de registro - Email: %s, Rol: %s", req.Email, req.Role)

	// Verificar email duplicado
	count, err := facades.Orm().Query().Model(&models.User{}).
		Where("email = ?", req.Email).Count()
	if err != nil {
		facades.Log().Errorf("Error al verificar email duplicado: %v", err)
		return ctx.Render("auth/register", fiber.Map{
			"title":       "Crear cuenta",
			"flash_error": "Error al verificar los datos", "csrfToken": csrf.TokenFromContext(ctx),
		}, "layouts/base")
	}

	if count > 0 {
		facades.Log().Warningf("Registro fallido - Email ya registrado: %s", req.Email)
		return ctx.Render("auth/register", fiber.Map{
			"title":       "Crear cuenta",
			"flash_error": "El email ya está registrado", "csrfToken": csrf.TokenFromContext(ctx),
		}, "layouts/base")
	}

	user, err := c.userService.CreateUserWithRole(&req)
	if err != nil {
		facades.Log().Errorf("Error al crear usuario: %v", err)
		return ctx.Render("auth/register", fiber.Map{
			"title": "Crear cuenta", "csrfToken": csrf.TokenFromContext(ctx),
			"flash_error": "No se pudo crear la cuenta. Comprueba tus datos, usa una contraseña de 8 a 72 caracteres y un teléfono no registrado.",
		}, "layouts/base")
	}

	log.Printf("Usuario registrado exitosamente - ID: %d, Email: %s", user.ID, user.Email)

	sess := session.FromContext(ctx)
	if sess == nil {
		facades.Log().Error("session.FromContext devolvió nil en Register")
		return ctx.Render("auth/register", fiber.Map{
			"title":       "Crear cuenta",
			"flash_error": "Error interno al crear la sesión", "csrfToken": csrf.TokenFromContext(ctx),
		}, "layouts/base")
	}

	if err := sess.Regenerate(); err != nil {
		return fiber.ErrInternalServerError
	}
	if _, err := services.NewHalconService().EnsurePersonal(user.ID); err != nil {
		return fiber.NewError(500, "No se pudo preparar tu halcón")
	}
	sess.Set("user_id", user.ID)
	sess.Set("role", user.Role)
	log.Println(sess.Get("user_id"))
	return ctx.Redirect().To("/profile")
}

// ShowLogin - GET /login
func (c *AuthController) ShowLogin(ctx fiber.Ctx) error {
	facades.Log().Debug("Mostrando formulario de login")
	return ctx.Render("auth/login", fiber.Map{
		"title": "Iniciar sesión", "csrfToken": csrf.TokenFromContext(ctx),
	})
}

// Login - POST /login
func (c *AuthController) Login(ctx fiber.Ctx) error {
	email := strings.ToLower(strings.TrimSpace(ctx.FormValue("email")))
	password := ctx.FormValue("password")

	log.Printf("Intento de login - Email: %s", email)

	if email == "" || password == "" {
		facades.Log().Warning("Login fallido - Email o contraseña vacíos")
		return ctx.Render("auth/login", fiber.Map{
			"title":       "Iniciar sesión",
			"flash_error": "Email y contraseña son obligatorios", "csrfToken": csrf.TokenFromContext(ctx),
		}, "layouts/base")
	}

	// 1. USAR FirstOrFail PARA QUE LANCE ERROR SI NO EXISTE
	var user models.User
	if err := facades.Orm().Query().Model(&models.User{}).
		Where("email = ?", email).FirstOrFail(&user); err != nil {
		facades.Log().Warningf("Login fallido - Usuario no encontrado: %s", email)
		return ctx.Render("auth/login", fiber.Map{
			"title":       "Iniciar sesión",
			"flash_error": "Credenciales inválidas", "csrfToken": csrf.TokenFromContext(ctx),
		}, "layouts/base")
	}

	log.Printf("Usuario encontrado - ID: %d, Email: %s", user.ID, user.Email)

	// 2. COMPARAR CONTRASEÑA
	if !user.Status {
		return fiber.ErrUnauthorized
	}
	if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(password)); err != nil {
		facades.Log().Warningf("Login fallido - Contraseña incorrecta para: %s", email)
		return ctx.Render("auth/login", fiber.Map{
			"title":       "Iniciar sesión",
			"flash_error": "Credenciales inválidas", "csrfToken": csrf.TokenFromContext(ctx),
		}, "layouts/base")
	}

	log.Printf("Contraseña verificada correctamente para: %s", email)

	// 3. CREAR SESIÓN
	sess := session.FromContext(ctx)
	if sess == nil {
		facades.Log().Error("session.FromContext devolvió nil en Login")
		return ctx.Render("auth/login", fiber.Map{
			"title":       "Iniciar sesión",
			"flash_error": "Error interno al crear la sesión", "csrfToken": csrf.TokenFromContext(ctx),
		}, "layouts/base")
	}

	// Regenerar ID de sesión para prevenir fijación de sesión
	if err := sess.Regenerate(); err != nil {
		facades.Log().Warningf("No se pudo regenerar ID de sesión: %v", err)
		return fiber.ErrInternalServerError
	}

	if _, err := services.NewHalconService().EnsurePersonal(user.ID); err != nil {
		return fiber.NewError(500, "No se pudo preparar tu halcón")
	}
	sess.Set("user_id", user.ID)
	sess.Set("role", user.Role)

	log.Printf("Login exitoso - ID: %d, Rol: %s, Email: %s", user.ID, user.Role, user.Email)
	log.Println(sess.Get("user_id"))
	return ctx.Redirect().To("/profile")
}

// Logout - POST /logout
func (c *AuthController) Logout(ctx fiber.Ctx) error {
	sess := session.FromContext(ctx)
	if sess != nil {
		userID := sess.Get("user_id")
		log.Printf("Logout - User ID: %v", userID)
		if err := sess.Destroy(); err != nil {
			return fiber.ErrInternalServerError
		}
	}
	return ctx.Redirect().To("/login")
}
