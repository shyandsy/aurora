/**
 * Role DTOs
 */

export interface Role {
  id: number;
  name: string;
  created: string;
  modified: string;
}

export interface CreateRoleRequest {
  name: string;
}

export interface UpdateRoleRequest {
  name: string;
}

