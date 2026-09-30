import { Injectable, inject } from '@angular/core';
import { TranslateService } from '@ngx-translate/core';
import { StorageService } from './storage.service';

export type SupportedLanguage = 'en' | 'zh-CN' | 'zh-TW';

@Injectable({
  providedIn: 'root'
})
export class I18nService {
  private readonly translateService = inject(TranslateService);
  private readonly storageService = inject(StorageService);

  private readonly LANGUAGE_KEY = 'admin_language';
  private readonly DEFAULT_LANGUAGE: SupportedLanguage = 'zh-CN';
  private readonly SUPPORTED_LANGUAGES: SupportedLanguage[] = ['zh-CN', 'en', 'zh-TW'];

  constructor() {
    const savedLanguage = this.getSavedLanguage();
    // Only use saved language if user has manually selected it, otherwise default to Chinese
    const initialLanguage = savedLanguage || this.DEFAULT_LANGUAGE;

    if (initialLanguage !== this.DEFAULT_LANGUAGE) {
      setTimeout(() => {
        this.setLanguage(initialLanguage);
      }, 100);
    }
    localStorage.setItem(this.LANGUAGE_KEY, initialLanguage);
  }

  /**
   * Get saved language from storage
   */
  private getSavedLanguage(): SupportedLanguage | null {
    const saved = localStorage.getItem(this.LANGUAGE_KEY);
    if (saved && this.SUPPORTED_LANGUAGES.includes(saved as SupportedLanguage)) {
      return saved as SupportedLanguage;
    }
    return null;
  }

  /**
   * Detect browser language
   */
  private detectBrowserLanguage(): SupportedLanguage | null {
    const browserLang = navigator.language || (navigator as any).userLanguage;

    if (this.SUPPORTED_LANGUAGES.includes(browserLang as SupportedLanguage)) {
      return browserLang as SupportedLanguage;
    }

    const langPrefix = browserLang.split('-')[0];
    if (langPrefix === 'zh') {
      return 'zh-CN';
    }

    return null;
  }

  /**
   * Set current language
   */
  setLanguage(lang: SupportedLanguage): void {
    if (!this.SUPPORTED_LANGUAGES.includes(lang)) {
      console.warn(`Unsupported language: ${lang}, falling back to ${this.DEFAULT_LANGUAGE}`);
      lang = this.DEFAULT_LANGUAGE;
    }

    this.translateService.use(lang).subscribe({
      next: () => { },
      error: () => { }
    });

    localStorage.setItem(this.LANGUAGE_KEY, lang);
  }

  /**
   * Get current language
   */
  getCurrentLanguage(): SupportedLanguage {
    const saved = this.getSavedLanguage();
    if (saved) {
      return saved;
    }
    return this.DEFAULT_LANGUAGE;
  }

  /**
   * Get supported languages
   */
  getSupportedLanguages(): SupportedLanguage[] {
    return [...this.SUPPORTED_LANGUAGES];
  }

  /**
   * Get language display name
   */
  getLanguageDisplayName(lang: SupportedLanguage): string {
    const names: Record<SupportedLanguage, string> = {
      'en': 'English',
      'zh-CN': '简体中文',
      'zh-TW': '繁體中文'
    };
    return names[lang] || lang;
  }
}

