// app.layout.* — layout chrome copy (runtime config in src/app.tsx)
export default {
  'app.layout.openapiDocs': 'OpenAPI Docs',

  // app.request.* — global request error messages (requestErrorConfig)
  'app.request.error.badGateway': 'Upstream service error',
  'app.request.error.conflict': 'Resource conflict',
  'app.request.error.default': 'Request failed',
  'app.request.error.exception': 'Request error, please try again later',
  'app.request.error.forbidden': 'Forbidden',
  'app.request.error.internalError': 'Internal server error',
  'app.request.error.invalidParams': 'Invalid request parameters',
  'app.request.error.invalidScope': 'The selected game environment is invalid. Please select again',
  'app.request.error.methodNotAllowed': 'Method not allowed',
  'app.request.error.moreDetails': '…and {count} more',
  'app.request.error.noResponse': 'No response received, please try again later',
  'app.request.error.notFound': 'Resource not found',
  'app.request.error.notImplemented': 'Not implemented',
  'app.request.error.rateLimited': 'Too many requests',
  'app.request.error.requestTooLarge': 'Request body too large',
  'app.request.error.unauthorized': 'Unauthorized',
  'app.request.error.unavailable': 'Service unavailable',
  'app.request.error.unprocessable': 'Unprocessable request',
};
