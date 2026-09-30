export interface MicroserviceTokenFeature {
  id: number;
  name: string;
  description: string;
  featureList: string[];
  created: string;
  modified: string;
}

export interface CreateMicroserviceTokenFeatureRequest {
  name: string;
  description: string;
  featureList: string[];
}

export interface UpdateMicroserviceTokenFeatureRequest {
  name?: string;
  description?: string;
  featureList?: string[];
}

export interface MicroserviceTokenFeatureToken {
  id: number;
  microserviceFeatureIds: number[];
  description: string;
  featureList: string[];
  token: string;
  issuer: string;
  expiresAt: string;
  expiresIn: number;
  status: string; // 'ENABLED' or 'DISABLED'
  created: string;
  modified: string;
}

export interface IssueMicroserviceTokenRequest {
  microserviceFeatureIds: number[];
}

export interface IssueMicroserviceTokenResponse {
  token: string;
  issuer: string;
  expiresAt: number;
  expiresIn: number;
  tokenRecordId: number;
}

export interface GetMicroserviceTokenTokensRequest {
  page: number;
  pageSize: number;
}

