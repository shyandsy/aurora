import { HttpErrorResponse } from '@angular/common/http';

/**
 * Extract error message from HTTP error response
 * API returns errors in unified format: { "message": "..." }
 */
export function extractErrorMessage(error: HttpErrorResponse | Error | any): string {
  // Handle HttpErrorResponse
  if (error instanceof HttpErrorResponse) {
    // API returns unified format: { "message": "..." }
    if (error.error?.message) {
      return error.error.message;
    }

    // Fallback to status-specific messages
    if (error.status === 401) {
      // 401: Unauthorized (could be login failure or expired token)
      return 'Unauthorized. Please check your credentials or login again.';
    } else if (error.status === 400) {
      return 'Invalid request. Please check your input.';
    } else if (error.status >= 500) {
      return 'Server error. Please try again later.';
    } else if (error.status === 0) {
      return 'Network error or request timeout. Please check your connection.';
    } else if (error.status) {
      return error.message || `Request failed with status ${error.status}.`;
    }

    // Unknown HTTP error
    return error.message || 'An unexpected error occurred. Please try again.';
  }

  // Handle Error objects
  if (error instanceof Error) {
    return error.message;
  }

  // Handle plain objects with message property
  if (error?.message && typeof error.message === 'string') {
    return error.message;
  }

  // Fallback
  return 'An unexpected error occurred. Please try again.';
}

