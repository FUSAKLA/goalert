# Progressive Web App (PWA) Support

GoAlert now supports Progressive Web App (PWA) functionality, allowing users to install the application on their devices for an app-like experience.

## Features

- **Installable**: Users can install GoAlert on their home screen (mobile) or desktop
- **Offline Support**: Basic offline functionality with cached resources
- **App-like Experience**: Runs in standalone mode without browser UI
- **Push Notifications**: Ready for future push notification integration

## Installation

### Mobile (iOS/Android)

**iOS (Safari):**
1. Open GoAlert in Safari
2. Tap the Share button
3. Tap "Add to Home Screen"
4. Name the app and tap "Add"

**Android (Chrome):**
1. Open GoAlert in Chrome
2. Tap the menu (⋮) in the upper right
3. Tap "Add to Home screen" or "Install app"
4. Confirm the installation

### Desktop

**Chrome/Edge:**
1. Open GoAlert in Chrome or Edge
2. Click the install icon (➕) in the address bar
3. Click "Install" in the dialog

**Or from the menu:**
1. Click the menu (⋮)
2. Select "Install GoAlert" or "Install app"

## How It Works

### Service Worker
The service worker (`sw.js`) provides offline caching and fast loading:
- Caches essential resources on first visit
- Serves cached content when offline
- Updates cache in the background

### Web App Manifest
The manifest (`manifest.json`) defines app metadata:
- Application name and icons
- Theme colors matching GoAlert branding
- Display mode (standalone)
- Start URL and scope

### Offline Page
When offline and no cached content is available, users see a branded offline page with retry functionality.

## Technical Details

### Files Added
- `web/src/app/public/manifest.json` - PWA manifest
- `web/src/app/public/sw.js` - Service worker
- `web/src/app/public/offline.html` - Offline fallback page

### Code Changes
- `web/index.html` - Added manifest link and service worker registration
- `web/handler.go` - Added routes for `/sw.js` and `/offline.html`
- `Makefile` - Updated to copy PWA files during build

### Cache Strategy
The service worker uses a "network first, cache fallback" strategy:
1. Try to fetch from network
2. Cache successful responses
3. On network failure, serve from cache
4. If no cache, show offline page

## Browser Support

PWA features are supported in:
- Chrome/Edge (Desktop & Mobile) - Full support
- Safari (iOS 11.3+) - Full support
- Firefox (Desktop & Android) - Full support
- Samsung Internet - Full support

## Future Enhancements

Potential future PWA features:
- Push notifications for alerts
- Background sync for offline actions
- Badge API for unread alert counts
- Share target API for alert sharing
