# Spec: Local Auth Frontend Pages

## Requirements

### R1: Login Page with Email/Password
The login page MUST display an email/password form alongside the existing OAuth buttons.

#### R1.1: Form Fields
- Email input: type="email", required, placeholder="correo@ejemplo.com"
- Password input: type="password", required, placeholder="••••••••"
- Submit button: "Iniciar sesión"

#### R1.2: Validation
- Email MUST match a valid email format.
- Password MUST NOT be empty.
- Validation errors MUST display inline below the respective field.

#### R1.3: Navigation Links
- Link to `/register`: "¿No tienes cuenta? Regístrate"
- Link to `/forgot-password`: "¿Olvidaste tu contraseña?"

### R2: Registration Page
The registration page MUST allow new users to create an account with name, email, and password.

#### R2.1: Form Fields
- Name input: type="text", required, placeholder="Tu nombre"
- Email input: type="email", required
- Password input: type="password", required, min length 8
- Submit button: "Crear cuenta"

#### R2.2: Validation
- All fields are required.
- Email MUST be valid format.
- Password MUST be at least 8 characters.

#### R2.3: Post-Registration
- On success, the user MUST be automatically logged in (cookies set by backend).
- The user MUST be redirected to `/` (home).

#### R2.4: Navigation Links
- Link to `/login`: "¿Ya tienes cuenta? Inicia sesión"

### R3: Forgot Password Page
The forgot password page MUST allow users to request a password reset email.

#### R3.1: Form Fields
- Email input: type="email", required
- Submit button: "Enviar enlace de recuperación"

#### R3.2: Security Behavior
- The page MUST show the same success message regardless of whether the email exists: "Si el correo existe, recibirás un enlace de recuperación."
- This prevents user enumeration attacks.

#### R3.3: Navigation Links
- Link to `/login`: "Volver al login"

### R4: Reset Password Page
The reset password page MUST allow users to set a new password using a token from the email.

#### R4.1: Token Handling
- The page MUST read the `token` query parameter from the URL.
- If no token is present, the page MUST show an error: "Enlace inválido o expirado."

#### R4.2: Form Fields
- New password input: type="password", required, min length 8
- Confirm password input: type="password", required
- Submit button: "Restablecer contraseña"

#### R4.3: Validation
- New password and confirm password MUST match.
- Password MUST be at least 8 characters.

#### R4.4: Post-Reset
- On success, show message: "Contraseña actualizada. Redirigiendo al login..."
- Redirect to `/login` after 3 seconds.

### R5: API Service Extension
The `apiService` MUST provide:
- `forgotPassword(email: string): Promise<ApiResponse<void>>`
- `resetPassword(token: string, newPassword: string): Promise<ApiResponse<void>>`

### R6: Auth Store Extension
The Zustand auth store MUST provide:
- `forgotPassword(email: string): Promise<void>`
- `resetPassword(token: string, newPassword: string): Promise<void>`

### R7: Routing
The React Router MUST support:
- `/login` → LoginPage (already exists, enhanced)
- `/register` → RegisterPage
- `/forgot-password` → ForgotPasswordPage
- `/reset-password` → ResetPasswordPage
- All four routes MUST be treated as public (no auth check redirect).

---

## Scenarios

### S1: User logs in with email and password
**Given** the user is on `/login`
**When** the user enters a valid email and password
**And** clicks "Iniciar sesión"
**Then** the auth store calls `apiService.login()`
**And** on success, the user is redirected to `/`
**And** the auth state shows `isAuthenticated: true`

### S2: User sees validation errors on login
**Given** the user is on `/login`
**When** the user clicks "Iniciar sesión" with empty fields
**Then** inline validation errors appear below the email and password fields
**And** no API call is made

### S3: User registers a new account
**Given** the user is on `/register`
**When** the user enters name, valid email, and password (8+ chars)
**And** clicks "Crear cuenta"
**Then** the auth store calls `apiService.register()`
**And** on success, the user is automatically logged in
**And** the user is redirected to `/`

### S4: User requests password reset
**Given** the user is on `/forgot-password`
**When** the user enters any email address
**And** clicks "Enviar enlace de recuperación"
**Then** the auth store calls `apiService.forgotPassword()`
**And** the UI shows: "Si el correo existe, recibirás un enlace de recuperación."
**And** this message is identical whether the email exists or not

### S5: User resets password with valid token
**Given** the user opens `/reset-password?token=abc123`
**When** the user enters a new password and matching confirmation
**And** clicks "Restablecer contraseña"
**Then** the auth store calls `apiService.resetPassword()`
**And** on success, the UI shows: "Contraseña actualizada. Redirigiendo al login..."
**And** after 3 seconds, the user is redirected to `/login`

### S6: User opens reset page without token
**Given** the user opens `/reset-password` without a token
**Then** the page shows an error: "Enlace inválido o expirado."
**And** no form is displayed
