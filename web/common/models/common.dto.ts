/**
 * Common DTOs shared across web/admin and web/customer
 */

export interface PagingResponse<T> {
  page: number;
  pageSize: number;
  total: number;
  totalPages: number;
  hasNext: boolean;
  hasPrev: boolean;
  items: T[];
}

