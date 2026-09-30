import { Component, input, output } from '@angular/core';
import { CommonModule } from '@angular/common';
import { TranslateModule } from '@ngx-translate/core';

@Component({
  selector: 'app-pagination',
  standalone: true,
  imports: [CommonModule, TranslateModule],
  templateUrl: './pagination.component.html',
  styleUrl: './pagination.component.css'
})
export class PaginationComponent {
  // Inputs
  currentPage = input.required<number>();
  totalPages = input.required<number>();
  total = input.required<number>();
  pageSize = input.required<number>();
  hasNext = input.required<boolean>();
  hasPrev = input.required<boolean>();

  // Outputs
  pageChange = output<number>();

  /**
   * Go to next page
   */
  nextPage(): void {
    if (this.hasNext()) {
      this.pageChange.emit(this.currentPage() + 1);
    }
  }

  /**
   * Go to previous page
   */
  prevPage(): void {
    if (this.hasPrev()) {
      this.pageChange.emit(this.currentPage() - 1);
    }
  }

  /**
   * Go to specific page. Accepts the windowed list entries (number | "…" sentinel);
   * the "…" gap markers are ignored via the typeof guard.
   */
  goToPage(page: number | string): void {
    if (typeof page === 'number' && page >= 1 && page <= this.totalPages()) {
      this.pageChange.emit(page);
    }
  }

  /** Jump straight to the first page. */
  firstPage(): void {
    if (this.currentPage() !== 1) {
      this.pageChange.emit(1);
    }
  }

  /** Jump straight to the last page (fixes "can't reach the last page" with huge totals). */
  lastPage(): void {
    const last = this.totalPages();
    if (this.currentPage() !== last) {
      this.pageChange.emit(last);
    }
  }

  // Sentinel used for the "…" gap markers in the windowed page list.
  readonly ellipsis = '…';

  /**
   * Windowed page list: always show first + last, a small window around the current page,
   * and "…" for the gaps. Prevents rendering thousands of buttons when totalPages is large
   * (e.g. 2106 pages) — which previously overflowed the bar and hid the last page.
   * Returns a mix of page numbers and the `ellipsis` sentinel.
   */
  getPageNumbers(): (number | string)[] {
    const total = this.totalPages();
    const current = this.currentPage();
    const delta = 2; // how many pages to show on each side of the current page

    // Few enough pages to show them all — no windowing needed.
    if (total <= 7) {
      return Array.from({ length: total }, (_, i) => i + 1);
    }

    const pages: (number | string)[] = [1];
    const left = Math.max(2, current - delta);
    const right = Math.min(total - 1, current + delta);

    if (left > 2) {
      pages.push(this.ellipsis);
    }
    for (let i = left; i <= right; i++) {
      pages.push(i);
    }
    if (right < total - 1) {
      pages.push(this.ellipsis);
    }
    pages.push(total);
    return pages;
  }

  /**
   * Get min value (for template)
   */
  min(a: number, b: number): number {
    return Math.min(a, b);
  }
}

