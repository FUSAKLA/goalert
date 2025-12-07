// GoAlert Service Worker for PWA support
// Provides offline caching and app-like functionality

const CACHE_NAME = 'goalert-cache-v1';
const urlsToCache = [
    '/',
    '/static/app.css',
    '/static/app.js',
    '/static/favicon-16.png',
    '/static/favicon-32.png',
    '/static/favicon-64.png',
    '/static/favicon-128.png',
    '/static/favicon-192.png',
    '/offline.html'
];

// Install event - cache essential resources
self.addEventListener('install', function (event) {
    event.waitUntil(
        caches.open(CACHE_NAME)
            .then(function (cache) {
                console.log('GoAlert: Service worker caching essential resources');
                return cache.addAll(urlsToCache);
            })
    );
});

// Activate event - clean up old caches
self.addEventListener('activate', function (event) {
    event.waitUntil(
        caches.keys().then(function (cacheNames) {
            return Promise.all(
                cacheNames.map(function (cacheName) {
                    if (cacheName !== CACHE_NAME) {
                        console.log('GoAlert: Removing old cache', cacheName);
                        return caches.delete(cacheName);
                    }
                })
            );
        })
    );
});

// Fetch event - network first, fallback to cache
self.addEventListener('fetch', function (event) {
    event.respondWith(
        fetch(event.request)
            .then(function (response) {
                // Don't cache non-GET requests or non-successful responses
                if (event.request.method !== 'GET' || !response || response.status !== 200) {
                    return response;
                }

                // Clone the response
                const responseToCache = response.clone();

                // Cache the fetched response
                caches.open(CACHE_NAME)
                    .then(function (cache) {
                        cache.put(event.request, responseToCache);
                    });

                return response;
            })
            .catch(function () {
                // Network failed, try to get from cache
                return caches.match(event.request)
                    .then(function (response) {
                        if (response) {
                            return response;
                        }

                        // If no cache match, return offline page for navigation requests
                        if (event.request.mode === 'navigate') {
                            return caches.match('/offline.html');
                        }
                    });
            })
    );
});
