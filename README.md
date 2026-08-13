# Phone Control Platform — Production Web UI Prototype & Engineering Baseline

Official web client display name: **Phone Control Platform**  
Baseline Version: **v1.0.0-prototype**  
Architecture Reference: `Phone-Control-Platform-Blueprint.md`

---

## 🌟 Overview & Key Features

The **Phone Control Platform** provides a centralized, secure, low-latency web management system for remote Android device fleets (Phone Farms, QA Device Labs, Kiosks, Remote Support).

### Core Features Matrix

- **Aesthetics & Layout**: Seamless top header & left sidebar alignment without dividing borders, inverted inner rounded curve (`rounded-tl-3xl shadow-inner`), dual-language support (Tiếng Việt `vi-VN` & English `en-US`).
- **Device Management**: Virtualized filterable device table (`/app/devices`), multi-device screen grid with dynamic tile scaling (`/app/devices/grid`), and detailed hardware spec gauges (`/app/devices/:id`).
- **Interactive Control**: Simulated remote screen touch/drag gestures `(x, y)`, 60-second exclusive Control Lease timer, global navigation keys (Home, Back, Recents), and Remote Text Input.
- **Capabilities Guard**: Automatic enforcement of APK capabilities. Sensitive actions (Reboot, Power, Proxy apply, Install APK) are disabled for standard non-root/non-ADB APK installations.
- **Android Agent Enrollment**: `POST /api/v1/agent-enrollments` mock contract generating single-use enrollment QR codes (`/app/agents`) with a 10-minute TTL expiration.
- **Realtime Engine**: WebSocket simulator (`/ws/v1`) for presence changes, telemetry updates, and command lifecycle state transitions (`pending` → `ack` → `executing` → `succeeded`).
- **Organization & RBAC**: Team role matrix (Owner, Admin, Manager, Operator, Viewer), System Audit Logs (`/app/audit`), Active WebRTC Stream Sessions (`/app/sessions`), and Enterprise Proxy Profiles (`/app/proxy`).
- **Security & Support**: Time-bound authorized Support Access Grant (`SupportAccessModal`) with explicit consent and ticket correlation (`support.access.granted`).
- **Infra Diagnostics**: System health readiness checks (`/health/ready`), DB connection pool metrics, Redis memory, and coturn TURN server relay monitoring (`/app/diagnostics`).
- **Commerce Preview**: Feature-flagged Rental Store (`/app/rental`) under `VITE_FEATURE_RENTAL_STORE`.

---

## 🛠️ Tech Stack

- **Framework**: React 18 + TypeScript (Strict Mode) + Vite
- **Styling**: Tailwind CSS + Custom Design Tokens
- **Icons**: Lucide React
- **State & Data**: TanStack React Query + Zustand (UI State Only) + React Hook Form + Zod
- **Internationalization**: i18next + react-i18next
- **Testing**: Vitest + Testing Library
- **Containerization**: Docker + Nginx + Caddy + Makefile

---

## 🚀 Quick Start Guide

### Prerequisites
- Node.js `v18+` or `v20+`
- npm `v9+` or `v10+`

### Installation & Development Server
```bash
# Install dependencies
npm install

# Start development server
npm run dev

# Run TypeScript type check
npm run typecheck

# Run test suite (Unit & E2E workflow integration tests)
npm run test

# Build production bundle
npm run build

# Preview production build locally
npm run preview
```

### Docker Container Deployment
```bash
# Start container cluster (Web + PostgreSQL + Redis + Coturn)
docker-compose up -d --build

# Stop container cluster
docker-compose down
```

---

## 🧪 Testing Matrix

Run the automated test suite with Vitest:
```bash
npx vitest run
```
Tests cover:
1. `MediaClient` WebRTC stream session lifecycle.
2. Zustand UI preference store isolation.
3. RBAC permission catalog validation.
4. Device entity DTO contract integrity.
5. End-to-End device control workflow integration.

---

## 📄 License & Compliance

All software components strictly adhere to the **Phone Control Platform** product blueprint. Unauthorized anti-detection or fake social engagement mechanisms are explicitly prohibited.
