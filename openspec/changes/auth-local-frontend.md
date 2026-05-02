# Change Proposal: Local Auth Frontend Pages

## Intent
Enable users to register, log in, and recover their password using email/password credentials through dedicated React pages, complementing the existing OAuth (Google/GitHub) flow.

## Scope

### In Scope
- **Frontend pages** (React + Tailwind):
  - Login page with email/password form (+ existing OAuth buttons)
  - Registration page with name/email/password form
  - Forgot password page (email input)
  - Reset password page (token + new password)
- **Frontend API service**: Add `forgotPassword` and `resetPassword` methods
- **Frontend routing**: Wire new pages into the React Router
- **Auth store (Zustand)**: Add `forgotPassword` and `resetPassword` actions
- **Backend verification**: Confirm all local auth endpoints work end-to-end

### Out of Scope
- Backend auth implementation (already complete: register, login, forgot-password, reset-password, JWT, cookies, bcrypt)
- Email SMTP configuration (uses NoopEmailSender in dev)
- OAuth flow changes

## Affected Modules
| Module | Impact |
|--------|--------|
| `r3-chat/src/components/auth/LoginPage.tsx` | Add email/password form |
| `r3-chat/src/components/auth/RegisterPage.tsx` | New page |
| `r3-chat/src/components/auth/ForgotPasswordPage.tsx` | New page |
| `r3-chat/src/components/auth/ResetPasswordPage.tsx` | New page |
| `r3-chat/src/services/api.ts` | Add forgot/reset methods |
| `r3-chat/src/stores/auth.store.ts` | Add forgot/reset actions |
| `r3-chat/src/components/routing/` | Add new routes |
| `r3-chat/src/App.tsx` | Add public routes |

## Approach
Reuse the existing dark-themed login page design (gradients, glassmorphism, motion). Each new page follows the same visual pattern. Forms use controlled inputs with validation. The auth store already supports `login` and `register`; we extend it minimally.

## Rollback Plan
- Revert frontend commits
- Remove new routes from router
- No backend changes required

## Risks
| Risk | Likelihood | Mitigation |
|------|------------|------------|
| Cookie/auth issues in dev | Medium | Test with `withCredentials: true` and verify CORS |
| Email not sending in dev | Low | Use NoopEmailSender; log reset URLs to console |
