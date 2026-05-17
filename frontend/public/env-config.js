/**
 * Runtime environment configuration.
 * Values here can be overridden at deploy-time by modifying this file
 * or by mounting a custom env-config.js in the Docker container.
 *
 * This file is loaded BEFORE the app bundle via <script> in index.html.
 */
window.__ENV__ = {
  /**
   * API_URL: Base URL for the Bytebase API backend.
   * - Empty string (default): Same-origin (embedded mode, no CORS).
   * - Example: 'https://api.bytebase.example.com' for standalone frontend.
   */
  API_URL: '',

  /**
   * AUTH_MODE: Authentication token delivery method.
   * - 'cookie' (default): HttpOnly cookies (embedded mode, same-origin).
   * - 'token': Bearer token in Authorization header (standalone mode, cross-origin).
   */
  AUTH_MODE: 'cookie',
};
