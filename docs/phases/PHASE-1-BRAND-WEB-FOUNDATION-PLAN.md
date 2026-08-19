# PHASE 1 — BRAND PACKAGE + WEB FOUNDATION (IMPLEMENTATION PLAN)

## 1. CodeGraph Impact Inspection (Current Frontend)
- **AppShell**: Currently monolithic in `src/components/layout/Layout.tsx`. Needs splitting into `ClientAppShell.tsx` and `AdminAppShell.tsx`.
- **Sidebar & Header**: `Sidebar.tsx` and `Header.tsx` currently contain mixed logic for devices, operations, team, etc.
- **Router**: `src/router.tsx` is flat under `/app`. Needs restructuring into `/client` and `/admin` or logical route separation for Client and Admin domains.
- **UI primitives**: Not explicitly separated into a `ui/` package. Need to extract/create `Button`, `Card`, `Modal`, `Toast`, `Badge`, `EmptyState`, `LoadingState`, `ErrorBoundary`.
- **Zustand/UI state**: `src/stores/useUiStore.ts` exists for sidebar collapse and feature toggles.
- **AuthContext & Guards**: `src/context/AuthContext.tsx`, `RouteGuard.tsx`, `PermissionGuard.tsx` exist and are functioning.
- **Device control**: `DeviceControlModal.tsx` and others exist in `src/components/devices/`. Must not rewrite these.
- **Responsive behavior**: Handled via basic Tailwind classes (`md:hidden`, etc.).

## 2. Target Architecture for Phase 1
We will introduce a monorepo-style package structure within the repository to isolate components logically, even if we still build via Vite:
```text
packages/
├── brand/
│   ├── assets/     # Contains the approved CloudPhoneRental logo
│   └── src/        # Brand tokens, colors, typography
└── ui/
    └── src/
        ├── button/
        ├── card/
        ├── modal/
        ├── toast/
        ├── badge/
        ├── empty/
        ├── loading/
        └── error/
```
Inside `src/`:
```text
src/
├── components/layout/
│   ├── ClientAppShell.tsx
│   ├── ClientSidebar.tsx
│   ├── ClientHeader.tsx
│   ├── AdminAppShell.tsx
│   ├── AdminSidebar.tsx
│   └── MobileNavigation.tsx
├── navigation/
│   ├── client-navigation.ts
│   └── admin-navigation.ts
```

## 3. Slice Breakdown

### SLICE 1.1 — BRAND + DESIGN SYSTEM (ACTIVE)
- **Goal:** Set up `packages/brand` and `packages/ui` with Design Tokens, Logo integration, and shared UI primitives (Button, Card, Badge, Toast, Modal/Dialog, Loading, Empty State, Error State).
- **Style Guidelines:** Modern premium SaaS, minimal text, spacious, rounded corners, white app bg, very light gray workspace, emerald brand accent, subtle borders/shadows. No oversized glassmorphism.
- **Note:** Blocked on the exact Owner-approved logo file if not provided yet.

### SLICE 1.2 — CLIENT APP SHELL
- **Goal:** Build the separate `ClientAppShell.tsx` with specific Header, Sidebar (4 items: Store, Devices, Wallet, Docs), and responsive Mobile Navigation.

### SLICE 1.3 — CLIENT PAGE SHELLS
- **Goal:** Build beautiful, empty/mocked page shells for the 4 Client pages. No backend integration.

### SLICE 1.4 — ADMIN FOUNDATION
- **Goal:** Build the separate `AdminAppShell.tsx` and full navigation tree with empty operational page shells.
