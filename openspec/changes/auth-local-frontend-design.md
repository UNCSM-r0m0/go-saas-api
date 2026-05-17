# Design: Local Auth Frontend Pages

## Architecture Overview

```
┌─────────────────────────────────────────────────────────────┐
│  React Router                                               │
│  /login → LoginPage                                         │
│  /register → RegisterPage                                   │
│  /forgot-password → ForgotPasswordPage                      │
│  /reset-password → ResetPasswordPage                        │
└─────────────────────────────────────────────────────────────┘
                              │
              ┌───────────────┼───────────────┐
              ▼               ▼               ▼
┌─────────────────┐ ┌─────────────────┐ ┌─────────────────┐
│  useAuthStore   │ │   apiService    │ │  useNavigate    │
│  (Zustand)      │ │   (axios)       │ │  (react-router) │
└─────────────────┘ └─────────────────┘ └─────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────┐
│  API Gateway (localhost:3000)                               │
│  POST /auth/register                                        │
│  POST /auth/login                                           │
│  POST /auth/forgot-password                                 │
│  POST /auth/reset-password                                  │
└─────────────────────────────────────────────────────────────┘
```

## Component Design

### Shared: AuthPageLayout
All four pages share a common layout:
- Full-screen dark gradient background (same as current LoginPage)
- Left side: marketing copy (features, providers) — only on desktop
- Right side: glassmorphism card with form
- Motion: framer-motion fade-in animations

### LoginPage (`src/components/auth/LoginPage.tsx`)
**Changes to existing file:**
- Add email/password form above OAuth buttons
- Add divider: "— o continúa con —"
- Add navigation links at bottom

```tsx
// New state
const [email, setEmail] = useState('');
const [password, setPassword] = useState('');
const [formErrors, setFormErrors] = useState<{email?: string; password?: string}>();

// New handler
const handleLocalLogin = async (e: React.FormEvent) => {
  e.preventDefault();
  // validate
  await login(email, password);
  navigate('/', { replace: true });
};
```

### RegisterPage (`src/components/auth/RegisterPage.tsx`)
- New file, same layout pattern
- Form fields: name, email, password
- On success: `register()` from auth store, then redirect to `/`

### ForgotPasswordPage (`src/components/auth/ForgotPasswordPage.tsx`)
- New file, same layout pattern
- Single email field
- Success state shows message (no auto-redirect)

### ResetPasswordPage (`src/components/auth/ResetPasswordPage.tsx`)
- New file, same layout pattern
- Reads `token` from `useSearchParams()`
- Two password fields (new + confirm)
- On success: show message + setTimeout redirect to `/login`

## API Service Extension (`src/services/api.ts`)

```typescript
async forgotPassword(email: string): Promise<ApiResponse<void>> {
    const response = await this.api.post('/auth/forgot-password', { email });
    return response.data;
}

async resetPassword(token: string, newPassword: string): Promise<ApiResponse<void>> {
    const response = await this.api.post('/auth/reset-password', { token, new_password: newPassword });
    return response.data;
}
```

## Auth Store Extension (`src/stores/auth.store.ts`)

```typescript
forgotPassword: async (email: string) => {
    set({ isLoading: true, error: null });
    try {
        await apiService.forgotPassword(email);
        set({ isLoading: false });
    } catch (error) {
        set({ isLoading: false, error: 'Error al enviar el correo' });
        throw error;
    }
},

resetPassword: async (token: string, newPassword: string) => {
    set({ isLoading: true, error: null });
    try {
        await apiService.resetPassword(token, newPassword);
        set({ isLoading: false });
    } catch (error) {
        set({ isLoading: false, error: 'Error al restablecer la contraseña' });
        throw error;
    }
},
```

## Routing Changes

### App.tsx
Add new paths to `publicRoutes`:
```typescript
const publicRoutes = ['/privacy', '/auth/callback', '/payment/success', '/payment/cancel', '/login', '/register', '/forgot-password', '/reset-password'];
```

### Router component
Add routes:
```tsx
<Route path="/register" element={<RegisterPage />} />
<Route path="/forgot-password" element={<ForgotPasswordPage />} />
<Route path="/reset-password" element={<ResetPasswordPage />} />
```

## State Management

| Page | Local State | Store Action |
|------|------------|--------------|
| Login | email, password, errors | `login(email, password)` |
| Register | name, email, password, errors | `register(name, email, password)` |
| Forgot | email, submitted | `forgotPassword(email)` |
| Reset | password, confirm, errors | `resetPassword(token, password)` |

## Error Handling

- **400 Bad Request**: Show validation errors inline
- **401 Unauthorized** (login): Show "Credenciales inválidas"
- **409 Conflict** (register): Show "Ya existe una cuenta con este correo"
- **500 Server Error**: Show generic "Error del servidor. Intenta de nuevo."

## Security Considerations

- Passwords are never stored in state after submission
- Token in reset URL is read-only, never persisted
- Forms prevent default submission to avoid page reload
- `withCredentials: true` ensures cookies are sent/received
