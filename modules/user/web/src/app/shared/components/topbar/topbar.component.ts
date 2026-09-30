import { Component, inject, signal } from '@angular/core';
import { CommonModule } from '@angular/common';
import { TranslateModule, TranslateService } from '@ngx-translate/core';
import { StorageService } from '../../../core/services/storage.service';
import { AuthService } from '../../../core/services/auth.service';
import { I18nService, SupportedLanguage } from '../../../core/services/i18n.service';
import { User } from '../../../shared/models/auth.dto';

@Component({
  selector: 'app-topbar',
  standalone: true,
  imports: [CommonModule, TranslateModule],
  templateUrl: './topbar.component.html',
  styleUrl: './topbar.component.css'
})
export class TopbarComponent {
  private readonly storageService = inject(StorageService);
  private readonly authService = inject(AuthService);
  private readonly i18nService = inject(I18nService);
  readonly translate = inject(TranslateService);

  showLanguageDropdown = signal(false);

  /**
   * Get current user email
   */
  getCurrentUserEmail(): string {
    const user = this.storageService.getUser() as User | null;
    return user?.email || '';
  }

  /**
   * Get current language
   */
  get currentLanguage(): SupportedLanguage {
    return this.i18nService.getCurrentLanguage();
  }

  /**
   * Get supported languages
   */
  get supportedLanguages(): SupportedLanguage[] {
    return this.i18nService.getSupportedLanguages();
  }

  /**
   * Get language display name
   */
  getLanguageDisplayName(lang: SupportedLanguage): string {
    return this.i18nService.getLanguageDisplayName(lang);
  }

  /**
   * Switch language
   */
  switchLanguage(lang: SupportedLanguage): void {
    this.i18nService.setLanguage(lang);
    this.showLanguageDropdown.set(false);
  }

  /**
   * Handle logout —— 交给 AuthService:先调后端拉黑 token,再清本地并整页刷新。
   */
  logout(): void {
    this.authService.logout();
  }
}

