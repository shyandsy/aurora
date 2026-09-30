/**
 * RoleFeature DTOs
 */

export interface RoleFeature {
  id: number;
  roleId: number;
  featureId: number;
  created: string;
  modified: string;
}

export interface CreateRoleFeatureRequest {
  roleId: number;
  featureId: number;
}

