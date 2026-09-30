/**
 * User DTOs
 */

import { Role } from './role.dto';

export interface User {
  id: number;
  email: string;
  roleId: number;
  status: number;
  created: string;
  modified: string;
  role?: Role;
  features: string[];
}

export interface CreateUserRequest {
  email: string;
  password: string;
  roleId: number;
}

export interface UpdateUserRequest {
  email?: string;
  password?: string;
  roleId?: number;
  status?: number;
}

export interface PagingRequest {
  page: number;
  pageSize: number;
}

