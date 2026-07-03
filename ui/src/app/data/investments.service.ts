import { HttpClient } from '@angular/common/http';
import { Injectable } from '@angular/core';
import { map } from 'rxjs';

import {
  CreateBulkInvestmentOperationPayload,
  CreateInvestmentAssetPayload,
  CreateInvestmentOperationMirrorPayload,
  CreateInvestmentOperationMirrorsBulkPayload,
  CreateInvestmentOperationPayload,
  CreateInvestmentPortfolioPayload,
  ImportInvestmentOperationsPayload,
  ImportInvestmentOperationsResponse,
  InvestmentAsset,
  InvestmentOperation,
  InvestmentPortfolio,
  InvestmentPortfolioAnalysis,
  InvestmentPortfolioSuggestion,
  InvestmentPosition,
  InvestmentPositionQuote,
  SaveInvestmentPortfolioAssetPayload,
  UpdateInvestmentAssetPayload,
  UpdateInvestmentOperationPayload,
  UpdateInvestmentPortfolioPayload,
} from '../shared/models';

@Injectable({ providedIn: 'root' })
export class InvestmentsService {
  constructor(private readonly http: HttpClient) {}

  listAssets() {
    return this.http
      .get<{ assets: InvestmentAsset[] }>('/api/investments/assets')
      .pipe(map((res) => res.assets ?? []));
  }

  createAsset(payload: CreateInvestmentAssetPayload) {
    return this.http
      .post<{ asset: InvestmentAsset }>('/api/investments/assets', payload)
      .pipe(map((res) => res.asset));
  }

  updateAsset(code: string, payload: UpdateInvestmentAssetPayload) {
    return this.http
      .patch<{ asset: InvestmentAsset }>(`/api/investments/assets/${code}`, payload)
      .pipe(map((res) => res.asset));
  }

  refreshAssetMetadata(code: string) {
    return this.http
      .post<{ asset: InvestmentAsset }>(`/api/investments/assets/${code}/refresh-metadata`, {})
      .pipe(map((res) => res.asset));
  }

  refreshMissingAssetMetadata() {
    return this.http
      .post<{ updated: number }>('/api/investments/assets/refresh-missing-metadata', {})
      .pipe(map((res) => res.updated ?? 0));
  }

  listPortfolios() {
    return this.http
      .get<{ portfolios: InvestmentPortfolio[] }>('/api/investments/portfolios')
      .pipe(map((res) => res.portfolios ?? []));
  }

  createPortfolio(payload: CreateInvestmentPortfolioPayload) {
    return this.http
      .post<{ portfolio: InvestmentPortfolio }>('/api/investments/portfolios', payload)
      .pipe(map((res) => res.portfolio));
  }

  updatePortfolio(code: string, payload: UpdateInvestmentPortfolioPayload) {
    return this.http
      .patch<{ portfolio: InvestmentPortfolio }>(`/api/investments/portfolios/${code}`, payload)
      .pipe(map((res) => res.portfolio));
  }

  deletePortfolio(code: string) {
    return this.http.delete<void>(`/api/investments/portfolios/${code}`);
  }

  analyzePortfolio(code: string) {
    return this.http
      .get<{ analysis: InvestmentPortfolioAnalysis }>(`/api/investments/portfolios/${code}/analysis`)
      .pipe(map((res) => res.analysis));
  }

  suggestPortfolioInvestment(code: string, investmentAmount: number) {
    return this.http
      .post<{ suggestion: InvestmentPortfolioSuggestion }>(`/api/investments/portfolios/${code}/suggestions`, {
        investment_amount: investmentAmount,
      })
      .pipe(map((res) => res.suggestion));
  }

  savePortfolioAsset(portfolioCode: string, assetCode: string, payload: SaveInvestmentPortfolioAssetPayload) {
    return this.http.put<void>(`/api/investments/portfolios/${portfolioCode}/assets/${assetCode}`, payload);
  }

  reorderPortfolioAssets(portfolioCode: string, codes: string[]) {
    return this.http.patch<void>(`/api/investments/portfolios/${portfolioCode}/assets/reorder`, { codes });
  }

  deletePortfolioAsset(portfolioCode: string, assetCode: string) {
    return this.http.delete<void>(`/api/investments/portfolios/${portfolioCode}/assets/${assetCode}`);
  }

  listOperations() {
    return this.http
      .get<{ operations: InvestmentOperation[] }>('/api/investments/operations')
      .pipe(map((res) => res.operations ?? []));
  }

  createOperation(payload: CreateInvestmentOperationPayload) {
    return this.http
      .post<{ operation: InvestmentOperation }>('/api/investments/operations', payload)
      .pipe(map((res) => res.operation));
  }

  createBulkOperations(payload: CreateBulkInvestmentOperationPayload) {
    return this.http
      .post<{ operations: InvestmentOperation[] }>('/api/investments/operations/bulk', payload)
      .pipe(map((res) => res.operations ?? []));
  }

  importOperations(payload: ImportInvestmentOperationsPayload) {
    return this.http.post<ImportInvestmentOperationsResponse>('/api/investments/import-operations', payload);
  }

  updateOperation(id: string, payload: UpdateInvestmentOperationPayload) {
    return this.http
      .patch<{ operation: InvestmentOperation }>(`/api/investments/operations/${id}`, payload)
      .pipe(map((res) => res.operation));
  }

  deleteOperation(id: string) {
    return this.http.delete<void>(`/api/investments/operations/${id}`);
  }

  createOperationMirror(id: string, payload: CreateInvestmentOperationMirrorPayload) {
    return this.http
      .post<{ operation: InvestmentOperation }>(`/api/investments/operations/${id}/mirror`, payload)
      .pipe(map((res) => res.operation));
  }

  createOperationMirrorsBulk(payload: CreateInvestmentOperationMirrorsBulkPayload) {
    return this.http
      .post<{ operations: InvestmentOperation[] }>('/api/investments/operations/mirror-bulk', payload)
      .pipe(map((res) => res.operations ?? []));
  }

  listPositions() {
    return this.http
      .get<{ positions: InvestmentPosition[] }>('/api/investments/positions')
      .pipe(map((res) => res.positions ?? []));
  }

  listPositionQuotes() {
    return this.http
      .get<{ quotes: InvestmentPositionQuote[] }>('/api/investments/position-quotes')
      .pipe(map((res) => res.quotes ?? []));
  }
}
