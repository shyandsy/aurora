import { Injectable, signal, effect } from '@angular/core';
import { environment } from '../../../environments/environment';

/**
 * Error notification service
 * Uses a modal component to display errors, preventing duplicate alerts
 */
@Injectable({
  providedIn: 'root'
})
export class ErrorService {
  // Signal to control error modal visibility and message
  private errorMessage = signal<string | null>(null);
  private isVisible = signal<boolean>(false);

  // Track last 403 error display time to prevent duplicate alerts
  private last403ErrorTime: number = 0;
  private readonly FORBIDDEN_ERROR_DEBOUNCE_MS = 3000; // 3 seconds

  constructor() {
    // Effect to automatically hide modal after configured time if visible
    effect(() => {
      if (this.isVisible()) {
        const timer = setTimeout(() => {
          this.hideError();
        }, environment.errorMessageAutoHideTime);
        return () => {
          clearTimeout(timer);
        };
      }
      return undefined;
    });
  }

  /**
   * Get error message signal (for component binding)
   */
  getErrorMessage() {
    return this.errorMessage.asReadonly();
  }

  /**
   * Get visibility signal (for component binding)
   */
  getVisibility() {
    return this.isVisible.asReadonly();
  }

  /**
   * Show error message
   */
  showError(message: string): void {
    // Only show if not already visible (prevents duplicate modals)
    if (!this.isVisible()) {
      this.errorMessage.set(message);
      this.isVisible.set(true);
    }
  }

  /**
   * Show 403 Forbidden error with debouncing
   * Prevents multiple modals when multiple API requests fail with 403 simultaneously
   */
  showForbiddenError(message: string): void {
    const now = Date.now();
    const timeSinceLastError = now - this.last403ErrorTime;

    // Check if we should show the error:
    // 1. Enough time has passed since last 403 error (debounce)
    // 2. No error modal is currently visible
    if (timeSinceLastError >= this.FORBIDDEN_ERROR_DEBOUNCE_MS && !this.isVisible()) {
      this.last403ErrorTime = now;
      this.showError(message);
    }
  }

  /**
   * Hide error modal
   */
  hideError(): void {
    this.isVisible.set(false);
    // Clear message after a short delay to allow animation
    setTimeout(() => {
      this.errorMessage.set(null);
    }, 200);
  }
}

