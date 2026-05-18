# Tasks: Local Auth Frontend Pages

## Phase 1: API Layer & Store

### 1.1 Extend apiService with forgot/reset methods
**File**: `r3-chat/src/services/api.ts`
**What**: Add `forgotPassword()` and `resetPassword()` methods.
**Acceptance**: Methods call `POST /auth/forgot-password` and `POST /auth/reset-password` correctly.

### 1.2 Extend auth store with forgot/reset actions
**File**: `r3-chat/src/stores/auth.store.ts`
**What**: Add `forgotPassword` and `resetPassword` actions to the Zustand store.
**Acceptance**: Store methods call apiService, set loading/error states.

### 1.3 Add forgot/reset to useAuth hook
**File**: `r3-chat/src/hooks/useAuth.ts`
**What**: Export the new store actions.
**Acceptance**: Hook returns `forgotPassword` and `resetPassword`.

## Phase 2: Shared Components

### 2.1 Create AuthPageLayout component
**File**: `r3-chat/src/components/auth/AuthPageLayout.tsx`
**What**: Extract the shared layout (gradient bg, left panel, glass card) from LoginPage.
**Acceptance**: All four auth pages can wrap their content in this layout.

### 2.2 Create AuthFormInput component
**File**: `r3-chat/src/components/auth/AuthFormInput.tsx`
**What**: Reusable input with label, error display, dark theme styling.
**Props**: `label`, `type`, `value`, `onChange`, `error`, `placeholder`, `required`
**Acceptance**: Used consistently across all forms.

### 2.3 Create AuthSubmitButton component
**File**: `r3-chat/src/components/auth/AuthSubmitButton.tsx`
**What**: Reusable submit button with loading state.
**Props**: `children`, `isLoading`, `disabled`
**Acceptance**: Shows spinner when `isLoading` is true.

## Phase 3: Pages

### 3.1 Enhance LoginPage with email/password form
**File**: `r3-chat/src/components/auth/LoginPage.tsx`
**What**: Add form above OAuth buttons; add links to /register and /forgot-password.
**Acceptance**: 
- Form validates email and password
- On submit, calls `login()` and redirects to `/`
- OAuth buttons still work

### 3.2 Create RegisterPage
**File**: `r3-chat/src/components/auth/RegisterPage.tsx`
**What**: New page with name/email/password form.
**Acceptance**:
- Validates all fields (name required, email valid, password >= 8)
- On submit, calls `register()` and redirects to `/`
- Link to `/login` at bottom

### 3.3 Create ForgotPasswordPage
**File**: `r3-chat/src/components/auth/ForgotPasswordPage.tsx`
**What**: Single email form.
**Acceptance**:
- Validates email format
- On submit, calls `forgotPassword()`
- Shows success message regardless of outcome
- Link to `/login`

### 3.4 Create ResetPasswordPage
**File**: `r3-chat/src/components/auth/ResetPasswordPage.tsx`
**What**: Token + new password form.
**Acceptance**:
- Reads `token` from URL query params
- Shows error if no token
- Validates password match and length
- On success, shows message and redirects to `/login` after 3s

## Phase 4: Routing

### 4.1 Add routes to AppRouter
**File**: `r3-chat/src/components/routing/AppRouter.tsx` (or equivalent)
**What**: Add `/register`, `/forgot-password`, `/reset-password` routes.
**Acceptance**: All routes render correct pages.

### 4.2 Update App.tsx public routes
**File**: `r3-chat/src/App.tsx`
**What**: Add new paths to `publicRoutes` array.
**Acceptance**: Visiting new routes does not trigger auth check redirect.

## Phase 5: Verification

### 5.1 Manual test: Register → Login → Chat
**Steps**:
1. Open `/register`, create account
2. Verify redirect to `/` and chat works
3. Logout
4. Login with same credentials
5. Verify redirect to `/`

### 5.2 Manual test: Forgot → Reset → Login
**Steps**:
1. Open `/forgot-password`, enter email
2. Check console for NoopEmailSender reset URL
3. Open reset URL
4. Set new password
5. Login with new password

### 5.3 Verify OAuth still works
**Steps**:
1. Login with Google
2. Verify redirect and cookies work
