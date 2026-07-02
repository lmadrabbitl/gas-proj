import { Component, HostListener, OnInit, computed, inject, signal } from '@angular/core';
import { FormsModule, FormBuilder, ReactiveFormsModule, Validators } from '@angular/forms';
import { RouterLink, RouterLinkActive } from '@angular/router';
import { forkJoin, of, switchMap } from 'rxjs';

import { InvestmentsService } from '../../data/investments.service';
import { ReferenceDataService } from '../../data/reference-data.service';
import { UserConfigService } from '../../data/user-config.service';
import { getApiErrorMessage } from '../../shared/api-error';
import { investmentAssetLabel, investmentAssetTypeLabel } from '../../shared/labels';
import { uiMessages } from '../../shared/messages';
import { MoneyVisibilityService } from '../../shared/money-visibility.service';
import { centsToDecimal, decimalToCents } from '../../shared/money';
import {
  InvestmentAsset,
  InvestmentAssetType,
  Category,
  InvestmentPortfolio,
  InvestmentPortfolioAnalysis,
  InvestmentPortfolioAnalysisRow,
  InvestmentPortfolioAsset,
  InvestmentSuggestionStrategy,
  InvestmentPortfolioSuggestion,
  InvestmentPosition,
} from '../../shared/models';
import { ToastService } from '../../shared/toast.service';

interface PortfolioAssetDraft {
  target: string;
  maxBuyPrice: string;
}

interface PortfolioDetailRow {
  asset: InvestmentPortfolioAsset;
  position: InvestmentPosition | null;
  analysis: InvestmentPortfolioAnalysisRow | null;
}

interface WatchedCategoryGroup {
  key: string;
  label: string | null;
  options: Category[];
}

@Component({
  selector: 'app-investment-portfolios',
  imports: [FormsModule, ReactiveFormsModule, RouterLink, RouterLinkActive],
  template: `
    <section class="page-header">
      <div>
        <p class="eyebrow">{{ messages.eyebrow }}</p>
        <h1>{{ messages.title }}</h1>
        <p class="page-subtitle">{{ messages.subtitle }}</p>
      </div>
      <div class="portfolio-page-actions">
        <button
          class="ghost-button settings-button"
          type="button"
          [title]="messages.actions.openSettings"
          [attr.aria-label]="messages.actions.openSettings"
          (click)="openSettings()"
        >
          <svg aria-hidden="true" viewBox="0 0 24 24">
            <path
              d="M19.14 12.94c.04-.31.06-.63.06-.94s-.02-.63-.06-.94l2.03-1.58a.5.5 0 0 0 .12-.63l-1.92-3.32a.5.5 0 0 0-.6-.22l-2.39.96a7.08 7.08 0 0 0-1.63-.94l-.36-2.54a.5.5 0 0 0-.5-.42h-3.84a.5.5 0 0 0-.49.42l-.36 2.54c-.58.23-1.12.54-1.63.94l-2.39-.96a.5.5 0 0 0-.6.22L2.7 8.85a.5.5 0 0 0 .12.63l2.03 1.58c-.04.31-.06.63-.06.94s.02.63.06.94L2.82 14.52a.5.5 0 0 0-.12.63l1.92 3.32a.5.5 0 0 0 .6.22l2.39-.96c.5.4 1.05.71 1.63.94l.36 2.54a.5.5 0 0 0 .49.42h3.84a.5.5 0 0 0 .5-.42l.36-2.54c.58-.23 1.12-.54 1.63-.94l2.39.96a.5.5 0 0 0 .6-.22l1.92-3.32a.5.5 0 0 0-.12-.63l-2.03-1.58ZM12 15.5A3.5 3.5 0 1 1 12 8.5a3.5 3.5 0 0 1 0 7Z"
            />
          </svg>
        </button>
        <button class="primary-button" type="button" (click)="openCreate()">{{ messages.create }}</button>
      </div>
    </section>

    <nav class="panel investment-subnav">
      <a routerLink="/investments/positions" routerLinkActive="active">{{ nav.positions }}</a>
      <a routerLink="/investments/assets" routerLinkActive="active">{{ nav.assets }}</a>
      <a routerLink="/investments/insert" routerLinkActive="active">{{ nav.insert }}</a>
      <a routerLink="/investments/operations" routerLinkActive="active">{{ nav.operations }}</a>
      <a routerLink="/investments/portfolios" routerLinkActive="active" [routerLinkActiveOptions]="{ exact: true }">
        {{ nav.portfolios }}
      </a>
    </nav>

    <section class="panel">
      @if (loading()) {
        <p class="state-message">{{ messages.loading }}</p>
      } @else if (portfolios().length === 0) {
        <p class="state-message">{{ messages.empty }}</p>
      } @else {
        <div class="table-wrap">
          <table>
            <thead>
              <tr>
                <th>{{ messages.columns.name }}</th>
                <th>{{ messages.columns.assets }}</th>
                <th>{{ messages.columns.totalPnl }}</th>
                <th>{{ messages.columns.dividends }}</th>
                <th>{{ messages.columns.totalPnlWithDividends }}</th>
                <th></th>
              </tr>
            </thead>
            <tbody>
              @for (portfolio of portfolios(); track portfolio.code) {
                <tr [class.selected-portfolio-row]="selectedPortfolioCode() === portfolio.code" (click)="selectPortfolioRow(portfolio.code)">
                  <td>
                    <button class="portfolio-row-button" type="button">
                      <span class="portfolio-row-icon" [class.expanded]="selectedPortfolioCode() === portfolio.code">
                        <svg aria-hidden="true" viewBox="0 0 24 24">
                          <path d="m9 6 6 6-6 6" />
                        </svg>
                      </span>
                      <span>{{ portfolio.name }}</span>
                    </button>
                  </td>
                  <td>{{ portfolio.assets.length }}</td>
                  <td [class.result-positive]="portfolioTotalPnlPercent(portfolio) !== null && portfolioTotalPnlPercent(portfolio)! > 0"
                      [class.result-negative]="portfolioTotalPnlPercent(portfolio) !== null && portfolioTotalPnlPercent(portfolio)! < 0">
                    {{ portfolioTotalPnlDisplay(portfolio) }}
                  </td>
                  <td class="amount-cell">{{ portfolioMatchedDividendsDisplay(portfolio) }}</td>
                  <td [class.result-positive]="portfolioTotalPnlWithDividendsPercent(portfolio) !== null && portfolioTotalPnlWithDividendsPercent(portfolio)! > 0"
                      [class.result-negative]="portfolioTotalPnlWithDividendsPercent(portfolio) !== null && portfolioTotalPnlWithDividendsPercent(portfolio)! < 0">
                    {{ portfolioTotalPnlWithDividendsDisplay(portfolio) }}
                  </td>
                  <td class="actions-cell" (click)="$event.stopPropagation()">
                    <button
                      class="icon-action"
                      type="button"
                      [title]="messages.actions.edit"
                      [attr.aria-label]="messages.actions.edit"
                      (click)="openEditPortfolio(portfolio)"
                    >
                      <svg aria-hidden="true" viewBox="0 0 24 24">
                        <path d="M3 17.25V21h3.75L17.8 9.94l-3.75-3.75L3 17.25Zm14.71-9.04a1.003 1.003 0 0 0 0-1.42l-2.5-2.5a1.003 1.003 0 0 0-1.42 0l-1.96 1.96 3.75 3.75 2.13-1.79Z" />
                      </svg>
                    </button>
                    <button
                      class="icon-action danger"
                      type="button"
                      [title]="messages.actions.remove"
                      [attr.aria-label]="messages.actions.remove"
                      (click)="removePortfolio(portfolio)"
                    >
                      <svg aria-hidden="true" viewBox="0 0 24 24">
                        <path d="M9 3h6l1 2h4v2H4V5h4l1-2Zm-1 6h2v10H8V9Zm6 0h2v10h-2V9Zm-9 0h14l-1 12H6L5 9Z" />
                      </svg>
                    </button>
                  </td>
                </tr>
              }
            </tbody>
          </table>
        </div>
      }
    </section>

    @if (selectedPortfolio(); as selected) {
      <section class="panel portfolio-detail-panel">
        <div class="panel-header portfolio-detail-header">
          <div>
            <p class="eyebrow">{{ messages.detail.eyebrow }}</p>
            <h2>{{ selected.name }}</h2>
            @if (selected.description) {
              <p>{{ selected.description }}</p>
            }
          </div>
        </div>

        <div class="portfolio-summary-grid">
          <article class="summary-card">
            <span>{{ messages.summary.assets }}</span>
            <strong>{{ selected.assets.length }}</strong>
          </article>
          <article class="summary-card">
            <span>{{ messages.summary.currentValue }}</span>
            <strong [class.loading-placeholder-text]="!selectedAnalysis()">
              {{ selectedAnalysis() ? money(selectedAnalysis()!.total_current_value) : messages.states.quotePending }}
            </strong>
          </article>
          <article class="summary-card">
            <span>{{ messages.summary.totalPnl }}</span>
            <strong [class.result-positive]="selectedAnalysisPnlPct() !== null && selectedAnalysisPnlPct()! > 0"
                    [class.result-negative]="selectedAnalysisPnlPct() !== null && selectedAnalysisPnlPct()! < 0">
              {{ selectedAnalysisPnlDisplay() }}
            </strong>
          </article>
          <article class="summary-card">
            <span>{{ messages.summary.minimumSuggestedInvestment }}</span>
            <strong [class.loading-placeholder-text]="isLoadingLabel(minimumSuggestedInvestmentDisplay())">{{ minimumSuggestedInvestmentDisplay() }}</strong>
          </article>
          <article class="summary-card">
            <span>{{ messages.summary.matchedDividends }}</span>
            <strong [class.loading-placeholder-text]="isLoadingLabel(matchedDividendsDisplay())">{{ matchedDividendsDisplay() }}</strong>
          </article>
        </div>

        <section class="analysis-panel">
          <div class="analysis-header">
            <div>
              <h3>{{ messages.detail.analysisTitle }}</h3>
              <p>{{ messages.detail.analysisSubtitle }}</p>
              @if (selectedAnalysisToleranceLabel()) {
                <p>{{ minimumSuggestedInvestmentHelpLabel() }}</p>
              }
            </div>
            @if (selectedAnalysisTargetTotal() !== null && selectedAnalysisTargetTotal() !== 100) {
              <p class="analysis-alert">{{ targetAlertLabel(selectedAnalysisTargetTotal()!) }}</p>
            }
          </div>
        </section>

        <section class="analysis-panel">
          <div class="analysis-header">
            <div>
              <h3>{{ messages.detail.incomeTitle }}</h3>
              <p>{{ messages.detail.incomeSubtitle }}</p>
              <p>{{ incomeSummaryHelpLabel() }}</p>
            </div>
          </div>

          @if (selectedAnalysisIncomeRows().length > 0) {
            <div class="table-wrap">
              <table class="portfolio-assets-table">
                <thead>
                  <tr>
                    <th>{{ messages.assetColumns.asset }}</th>
                    <th>{{ messages.assetColumns.dividends }}</th>
                    <th>{{ messages.assetColumns.transactionCount }}</th>
                  </tr>
                </thead>
                <tbody>
                  @for (row of selectedAnalysisIncomeRows(); track row.asset_code) {
                    <tr>
                      <td>{{ assetLabel(row.asset_code, row.asset_name) }}</td>
                      <td class="amount-cell">{{ money(row.amount) }}</td>
                      <td class="amount-cell">{{ row.transaction_count }}</td>
                    </tr>
                  }
                </tbody>
              </table>
            </div>
          } @else {
            <p class="section-note">—</p>
          }
        </section>

        <section class="analysis-panel">
          <div class="analysis-header">
            <div>
              <h3>{{ messages.detail.suggestionTitle }}</h3>
              <p>{{ messages.detail.suggestionSubtitle }}</p>
            </div>
          </div>

          <div class="suggestion-controls">
            <label class="suggestion-input">
              <span>{{ messages.suggestion.amount }}</span>
              <div class="money-input-shell" [class.invalid-input]="!isSuggestionAmountValid() && suggestionAmount().trim() !== ''">
                <span>R$</span>
                <input
                  inputmode="decimal"
                  [ngModel]="suggestionAmount()"
                  [placeholder]="messages.suggestion.amountPlaceholder"
                  (ngModelChange)="onSuggestionAmountInput($event)"
                  (blur)="formatSuggestionAmount()"
                />
              </div>
            </label>
            <button
              class="primary-button"
              type="button"
              [disabled]="suggestionLoading() || !isSuggestionAmountValid() || !selectedPortfolio()"
              (click)="generateSuggestion()"
            >
              {{ suggestionLoading() ? messages.states.suggestionLoading : messages.actions.generateSuggestion }}
            </button>
          </div>

          @if (selectedSuggestion()) {
            <div class="portfolio-summary-grid suggestion-summary-grid">
              <article class="summary-card">
                <span>{{ messages.suggestion.plannedSpend }}</span>
                <strong>{{ money(selectedSuggestion()!.planned_spend) }}</strong>
              </article>
              <article class="summary-card">
                <span>{{ messages.suggestion.cashRemainder }}</span>
                <strong>{{ money(selectedSuggestion()!.cash_remainder) }}</strong>
              </article>
            </div>

            <div class="table-wrap">
              <table class="portfolio-assets-table">
                <thead>
                  <tr>
                    <th>{{ messages.assetColumns.asset }}</th>
                    <th>{{ messages.assetColumns.currentPrice }}</th>
                    <th>{{ messages.assetColumns.currentAllocation }}</th>
                    <th>{{ messages.assetColumns.target }}</th>
                    <th>{{ messages.assetColumns.plannedShares }}</th>
                    <th>{{ messages.assetColumns.plannedSpend }}</th>
                    <th>{{ messages.assetColumns.projectedAllocation }}</th>
                    <th>{{ messages.assetColumns.recommendation }}</th>
                  </tr>
                </thead>
                <tbody>
                  @for (row of selectedSuggestion()!.rows; track row.asset_code) {
                    <tr>
                      <td>{{ assetLabel(row.asset_code, row.asset_name) }}</td>
                      <td class="amount-cell">{{ money(row.current_price) }}</td>
                      <td class="amount-cell">{{ basisPointsLabel(row.current_allocation_basis_point) }}</td>
                      <td class="amount-cell">{{ basisPointsLabel(row.target_allocation_basis_point) }}</td>
                      <td class="amount-cell">{{ row.buy_shares }}</td>
                      <td class="amount-cell">{{ row.planned_spend > 0 ? money(row.planned_spend) : '—' }}</td>
                      <td class="amount-cell">{{ basisPointsLabel(row.projected_allocation_basis_point) }}</td>
                      <td>{{ suggestionRecommendationLabel(row) }}</td>
                    </tr>
                  }
                </tbody>
              </table>
            </div>
          } @else if (suggestionLoading()) {
            <p class="section-note">{{ messages.states.suggestionLoading }}</p>
          } @else {
            <p class="section-note">{{ messages.states.suggestionEmpty }}</p>
          }
        </section>

        @if (portfolioAssetsEditMode() && assetPickerOpen()) {
          <section class="asset-picker">
            <div class="asset-picker-header">
              <div>
                <h3>{{ messages.detail.assetPickerTitle }}</h3>
                <p>{{ messages.detail.assetPickerSubtitle }}</p>
              </div>
              <div class="asset-picker-actions">
                <button
                  class="ghost-button"
                  type="button"
                  (click)="toggleCustomAssetForm()"
                >
                  {{ customAssetFormOpen() ? messages.actions.cancelCustomAsset : messages.actions.addCustomAsset }}
                </button>
                <button
                  class="primary-button"
                  type="button"
                  [disabled]="savingAssetSelection() || selectedAssetCodes().length === 0"
                  (click)="addSelectedAssets()"
                >
                  {{ savingAssetSelection() ? messages.actions.saving : messages.actions.addSelectedAssets }}
                </button>
              </div>
            </div>

            @if (customAssetFormOpen()) {
              <form class="custom-asset-form" [formGroup]="customAssetForm" (ngSubmit)="addCustomAsset()">
                <div class="custom-asset-form-copy">
                  <strong>{{ messages.detail.customAssetTitle }}</strong>
                  <p>{{ messages.detail.customAssetSubtitle }}</p>
                </div>
                <label>
                  {{ messages.form.assetCode }}
                  <input formControlName="code" />
                </label>
                <label>
                  {{ messages.form.assetName }}
                  <input formControlName="name" [placeholder]="messages.form.assetNamePlaceholder" />
                </label>
                <label>
                  {{ messages.form.assetType }}
                  <select formControlName="assetType">
                    <option value="STOCK">{{ assetType('STOCK') }}</option>
                    <option value="FII">{{ assetType('FII') }}</option>
                    <option value="ETF">{{ assetType('ETF') }}</option>
                  </select>
                </label>
                <button class="primary-button" type="submit" [disabled]="savingAssetSelection() || customAssetForm.invalid">
                  {{ savingAssetSelection() ? messages.actions.saving : messages.actions.confirmCustomAsset }}
                </button>
              </form>
            }

            @if (availableAssets().length === 0) {
              <p class="section-note">{{ messages.states.noAvailableAssets }}</p>
            } @else {
              <div class="asset-picker-grid">
                @for (asset of availableAssets(); track asset.code) {
                  <label class="checkbox-label asset-option">
                    <input
                      type="checkbox"
                      [ngModel]="isAssetSelected(asset.code)"
                      (ngModelChange)="setAssetSelection(asset.code, $event)"
                    />
                    <span>
                      <strong>{{ asset.code }}</strong>
                      <small>{{ asset.name }}</small>
                    </span>
                  </label>
                }
              </div>
            }
          </section>
        }

        <section class="analysis-panel">
          <div class="analysis-header">
            <div>
              <h3>{{ messages.detail.assetsTitle }}</h3>
              <p>{{ messages.detail.assetsSubtitle }}</p>
            </div>
            <div class="portfolio-detail-actions">
              @if (portfolioAssetsEditMode()) {
                <button class="ghost-button" type="button" (click)="toggleAssetPicker()">
                  {{ assetPickerOpen() ? messages.actions.closeAssetPicker : messages.actions.addAssets }}
                </button>
                <button class="ghost-button" type="button" (click)="cancelAssetEditMode()">
                  {{ messages.actions.cancel }}
                </button>
                <button
                  class="primary-button"
                  type="button"
                  [disabled]="savingAssetChanges() || !hasValidDrafts()"
                  (click)="saveAssetChanges()"
                >
                  {{ savingAssetChanges() ? messages.actions.saving : messages.actions.saveAssets }}
                </button>
              } @else {
                <button class="primary-button button-with-icon" type="button" [title]="messages.actions.editAssets" [attr.aria-label]="messages.actions.editAssets" (click)="enterAssetEditMode()">
                  <svg aria-hidden="true" viewBox="0 0 24 24">
                    <path d="M3 17.25V21h3.75L17.8 9.94l-3.75-3.75L3 17.25Zm14.71-9.04a1.003 1.003 0 0 0 0-1.42l-2.5-2.5a1.003 1.003 0 0 0-1.42 0l-1.96 1.96 3.75 3.75 2.13-1.79Z" />
                  </svg>
                  <span>{{ messages.actions.editAssets }}</span>
                </button>
              }
            </div>
          </div>

          @if (selected.assets.length === 0) {
            <p class="section-note">{{ messages.states.noAssets }}</p>
          } @else if (analysisLoading()) {
            <p class="section-note">{{ messages.states.analysisLoading }}</p>
          } @else if (!selectedAnalysis()) {
            <p class="section-note">{{ messages.states.analysisUnavailable }}</p>
          } @else {
            <div class="table-wrap">
              <table class="portfolio-assets-table">
                <thead>
                  <tr>
                    <th></th>
                    <th>{{ messages.assetColumns.asset }}</th>
                    <th>{{ messages.assetColumns.type }}</th>
                    <th>{{ messages.assetColumns.quantity }}</th>
                    <th>{{ messages.assetColumns.currentPrice }}</th>
                    <th>{{ messages.assetColumns.averagePrice }}</th>
                    <th>{{ messages.assetColumns.maxBuyPrice }}</th>
                    <th>{{ messages.assetColumns.pnl }}</th>
                    <th>{{ messages.assetColumns.target }}</th>
                    <th>{{ messages.assetColumns.currentAllocation }}</th>
                    <th>{{ messages.assetColumns.drift }}</th>
                    <th>{{ messages.assetColumns.recommendation }}</th>
                    <th></th>
                  </tr>
                </thead>
                <tbody>
                  @for (row of selectedPortfolioRows(); track row.asset.asset_code) {
                    <tr
                      [class.dragging-row]="draggingAssetCode() === row.asset.asset_code"
                      [class.drag-target-row]="dropTargetAssetCode() === row.asset.asset_code"
                      draggable="true"
                      (dragstart)="onAssetDragStart($event, row.asset)"
                      (dragover)="onAssetDragOver($event, row.asset)"
                      (drop)="onAssetDrop($event, row.asset)"
                      (dragend)="onAssetDragEnd()"
                    >
                      <td class="drag-cell">
                        <button
                          class="icon-action drag-handle"
                          type="button"
                          [title]="messages.actions.dragAssetTitle"
                          [attr.aria-label]="messages.actions.dragAssetAria"
                          tabindex="-1"
                        >↕</button>
                      </td>
                      <td>{{ assetLabel(row.asset.asset_code, row.asset.asset_name) }}</td>
                      <td>{{ assetType(row.asset.asset_type) }}</td>
                      <td class="amount-cell">{{ row.analysis?.current_quantity ?? row.position?.current_quantity ?? 0 }}</td>
                      <td class="amount-cell" [class.loading-placeholder-text]="isLoadingLabel(quoteDisplay(row))">{{ quoteDisplay(row) }}</td>
                      <td class="amount-cell" [class.loading-placeholder-text]="isLoadingLabel(averagePriceDisplay(row))">{{ averagePriceDisplay(row) }}</td>
                      <td>
                        @if (portfolioAssetsEditMode()) {
                          <div class="money-input-shell" [class.invalid-input]="!isMaxBuyPriceValid(row.asset.asset_code)">
                            <span>R$</span>
                            <input
                              class="amount-draft-input"
                              inputmode="decimal"
                              [ngModel]="assetDraft(row.asset.asset_code).maxBuyPrice"
                              (ngModelChange)="onMaxBuyPriceInput(row.asset.asset_code, $event)"
                              (blur)="formatMaxBuyPrice(row.asset.asset_code)"
                            />
                          </div>
                        } @else {
                          <span class="amount-cell">{{ row.asset.max_buy_price !== null && row.asset.max_buy_price !== undefined ? money(row.asset.max_buy_price) : '—' }}</span>
                        }
                      </td>
                      <td class="amount-cell"
                          [class.result-positive]="rowPnlPct(row) !== null && rowPnlPct(row)! > 0"
                          [class.result-negative]="rowPnlPct(row) !== null && rowPnlPct(row)! < 0">
                        <span [class.loading-placeholder-text]="isLoadingLabel(pnlDisplay(row))">{{ pnlDisplay(row) }}</span>
                      </td>
                      <td>
                        @if (portfolioAssetsEditMode()) {
                          <div class="suffix-input-shell" [class.invalid-input]="!isTargetValid(row.asset.asset_code)">
                            <input
                              class="amount-draft-input"
                              inputmode="decimal"
                              [ngModel]="assetDraft(row.asset.asset_code).target"
                              (ngModelChange)="onTargetInput(row.asset.asset_code, $event)"
                              (blur)="formatTarget(row.asset.asset_code)"
                            />
                            <span>%</span>
                          </div>
                        } @else {
                          <span class="amount-cell">{{ targetAllocationDisplay(row) }}</span>
                        }
                      </td>
                      <td class="amount-cell" [class.loading-placeholder-text]="isLoadingLabel(currentAllocationDisplay(row))">{{ currentAllocationDisplay(row) }}</td>
                      <td class="amount-cell"
                          [class.result-positive]="row.analysis !== null && row.analysis.allocation_drift_basis_point > 0"
                          [class.result-negative]="row.analysis !== null && row.analysis.allocation_drift_basis_point < 0">
                      <span [class.loading-placeholder-text]="isLoadingLabel(driftDisplay(row))">{{ driftDisplay(row) }}</span>
                      </td>
                      <td>{{ recommendationLabel(row) }}</td>
                      <td class="actions-cell">
                        @if (portfolioAssetsEditMode()) {
                          <button
                            class="icon-action danger"
                            type="button"
                            [title]="messages.actions.removeAsset"
                            [attr.aria-label]="messages.actions.removeAsset"
                            (click)="removePortfolioAsset(row.asset.asset_code)"
                          >
                            <svg aria-hidden="true" viewBox="0 0 24 24">
                              <path d="M9 3h6l1 2h4v2H4V5h4l1-2Zm-1 6h2v10H8V9Zm6 0h2v10h-2V9Zm-9 0h14l-1 12H6L5 9Z" />
                            </svg>
                          </button>
                        }
                      </td>
                    </tr>
                  }
                </tbody>
              </table>
            </div>
          }
        </section>
      </section>
    }

    @if (panelOpen()) {
      <aside class="side-panel">
        <div class="panel-header">
          <h2>{{ editingPortfolioCode() ? messages.form.editTitle : messages.form.createTitle }}</h2>
          <button class="ghost-button" type="button" (click)="closePanel()">{{ messages.actions.close }}</button>
        </div>
        <form class="form-stack" [formGroup]="portfolioForm" (ngSubmit)="savePortfolio()">
          <label>
            {{ messages.form.name }}
            <input formControlName="name" />
          </label>
          <label>
            {{ messages.form.description }}
            <textarea rows="3" formControlName="description"></textarea>
          </label>
          <button class="primary-button" type="submit" [disabled]="savingPortfolio() || portfolioForm.invalid">
            {{ savingPortfolio() ? messages.actions.saving : messages.actions.save }}
          </button>
        </form>
      </aside>
    }

    @if (settingsPanelOpen()) {
      <aside class="side-panel">
        <div class="panel-header">
          <div>
            <h2>{{ messages.settings.title }}</h2>
            <p>{{ messages.settings.description }}</p>
          </div>
          <button class="ghost-button" type="button" (click)="closeSettings()">{{ messages.actions.close }}</button>
        </div>
        <div class="form-stack">
          <div class="filter-field">
            <span class="filter-field-label">{{ messages.settings.watchedCategoriesLabel }}</span>
            <div class="multi-select" data-multi-select="watched-categories">
              <button class="multi-select-trigger" type="button" (click)="toggleWatchedCategoryMenu()">
                <span class="multi-select-value">{{ selectedWatchedCategoriesLabel() }}</span>
                <span class="multi-select-caret" aria-hidden="true">{{ watchedCategoryMenuOpen() ? '▴' : '▾' }}</span>
              </button>
              @if (watchedCategoryMenuOpen()) {
                <div class="multi-select-menu">
                  @for (group of watchedCategoryGroups(); track group.key) {
                    <div class="multi-select-group">
                      @if (group.label) {
                        <div class="multi-select-group-label">{{ group.label }}</div>
                      }
                      @for (category of group.options; track category.ID) {
                        <label class="multi-select-option">
                          <input
                            type="checkbox"
                            [checked]="isWatchedCategorySelected(category.ID)"
                            (change)="toggleWatchedCategorySelection(category.ID)"
                          />
                          <span>{{ category.Name }}</span>
                        </label>
                      }
                    </div>
                  }
                </div>
              }
            </div>
          </div>
          <p class="field-hint">{{ messages.settings.watchedCategoriesHint }}</p>
          <label>
            {{ messages.settings.strategyLabel }}
            <select [ngModel]="pendingSuggestionStrategy()" (ngModelChange)="pendingSuggestionStrategy.set($event)">
              <option value="BEST_NEXT_SHARE">{{ messages.settings.strategyBestNextShare }}</option>
              <option value="PROPORTIONAL_GAP">{{ messages.settings.strategyProportionalGap }}</option>
            </select>
          </label>
          <p class="field-hint">{{ strategyHintLabel() }}</p>
          <label>
            {{ messages.settings.toleranceLabel }}
            <div class="suffix-input-shell" [class.invalid-input]="!isRebalanceToleranceValid()">
              <input
                inputmode="decimal"
                [ngModel]="pendingRebalanceTolerance()"
                (ngModelChange)="onRebalanceToleranceInput($event)"
                (blur)="formatRebalanceTolerance()"
              />
              <span>%</span>
            </div>
          </label>
          <p class="field-hint">{{ messages.settings.toleranceHint }}</p>
          <button class="primary-button" type="button" [disabled]="!isRebalanceToleranceValid()" (click)="saveSettings()">
            {{ messages.actions.save }}
          </button>
        </div>
      </aside>
    }
  `,
  styles: [`
    .portfolio-page-actions {
      align-items: center;
      display: flex;
      gap: 12px;
      justify-content: flex-end;
      flex-wrap: wrap;
    }

    .investment-subnav {
      display: flex;
      gap: 12px;
      align-items: center;
      padding: 12px 16px;
      margin-bottom: 18px;
    }

    .investment-subnav a {
      color: var(--muted);
      text-decoration: none;
      padding: 8px 12px;
      border-radius: 999px;
    }

    .investment-subnav a.active {
      color: var(--text);
      background: var(--accent-soft);
    }

    .page-subtitle {
      margin: 6px 0 0;
      color: var(--muted);
    }

    .selected-portfolio-row td {
      background: var(--accent-soft);
    }

    .selected-portfolio-row {
      cursor: pointer;
    }

    .drag-cell {
      width: 48px;
    }

    .drag-handle {
      cursor: grab;
      min-width: 40px;
      padding: 0;
    }

    .dragging-row td {
      opacity: 0.5;
    }

    .drag-target-row td {
      background: var(--accent-soft);
    }

    .portfolio-row-button {
      display: inline-flex;
      align-items: center;
      gap: 10px;
      border: 0;
      background: transparent;
      padding: 0;
      color: inherit;
      font: inherit;
      font-weight: 700;
      text-align: left;
    }

    .portfolio-row-icon {
      display: inline-flex;
      align-items: center;
      justify-content: center;
      width: 18px;
      height: 18px;
      border-radius: 999px;
      background: rgba(42, 107, 92, 0.08);
      transition: transform 0.16s ease;
    }

    .portfolio-row-icon.expanded {
      transform: rotate(90deg);
    }

    .portfolio-row-icon svg,
    .portfolio-detail-actions svg {
      width: 14px;
      height: 14px;
      fill: none;
      stroke: currentColor;
      stroke-width: 2;
      stroke-linecap: round;
      stroke-linejoin: round;
    }

    .portfolio-detail-panel {
      margin-top: 18px;
      display: grid;
      gap: 18px;
    }

    .portfolio-detail-header {
      align-items: start;
      gap: 16px;
    }

    .portfolio-detail-header h2 {
      margin: 2px 0 6px;
    }

    .portfolio-detail-header p {
      margin: 0;
      color: var(--muted);
    }

    .portfolio-detail-actions {
      display: flex;
      align-items: center;
      justify-content: flex-end;
      gap: 10px;
      flex-wrap: wrap;
    }

    .button-with-icon {
      display: inline-flex;
      align-items: center;
      gap: 8px;
    }

    .portfolio-summary-grid {
      display: grid;
      grid-template-columns: repeat(auto-fit, minmax(180px, 1fr));
      gap: 12px;
    }

    .asset-picker {
      display: grid;
      gap: 14px;
      padding: 16px;
      border: 1px solid var(--border);
      border-radius: 16px;
      background: var(--surface-soft);
    }

    .analysis-panel {
      display: grid;
      gap: 8px;
      padding: 16px;
      border: 1px solid var(--border);
      border-radius: 16px;
      background: linear-gradient(135deg, rgba(24, 119, 103, 0.08), rgba(24, 119, 103, 0.02));
    }

    .analysis-header {
      display: flex;
      align-items: start;
      justify-content: space-between;
      gap: 16px;
      flex-wrap: wrap;
    }

    .analysis-header h3,
    .analysis-header p {
      margin: 0;
    }

    .analysis-header p {
      color: var(--muted);
    }

    .analysis-alert {
      margin: 0;
      color: #b54708;
      font-weight: 700;
    }

    .suggestion-controls {
      display: flex;
      align-items: end;
      gap: 12px;
      flex-wrap: wrap;
    }

    .suggestion-input {
      display: grid;
      gap: 6px;
      min-width: 220px;
    }

    .suggestion-input > span {
      color: var(--muted);
      font-size: 0.82rem;
      font-weight: 700;
    }

    .suggestion-summary-grid {
      margin-top: 4px;
    }

    .asset-picker-header {
      display: flex;
      align-items: start;
      justify-content: space-between;
      gap: 16px;
      flex-wrap: wrap;
    }

    .asset-picker-actions {
      display: flex;
      align-items: center;
      gap: 10px;
      flex-wrap: wrap;
      justify-content: flex-end;
    }

    .asset-picker-header h3 {
      margin: 0 0 4px;
    }

    .asset-picker-header p {
      margin: 0;
      color: var(--muted);
    }

    .asset-picker-grid {
      display: grid;
      grid-template-columns: repeat(auto-fit, minmax(220px, 1fr));
      gap: 10px;
    }

    .custom-asset-form {
      display: grid;
      grid-template-columns: minmax(220px, 1.4fr) repeat(3, minmax(160px, 1fr)) auto;
      gap: 12px;
      padding: 14px;
      border: 1px solid var(--border);
      border-radius: 14px;
      background: var(--surface);
      align-items: end;
    }

    .custom-asset-form-copy {
      display: grid;
      gap: 4px;
    }

    .custom-asset-form-copy strong,
    .custom-asset-form-copy p {
      margin: 0;
    }

    .custom-asset-form-copy p {
      color: var(--muted);
    }

    .asset-option {
      padding: 12px 14px;
      border: 1px solid var(--border);
      border-radius: 14px;
      background: var(--surface);
    }

    .asset-option span {
      display: grid;
      gap: 3px;
    }

    .asset-option small {
      color: var(--muted);
    }

    .result-positive {
      color: #1b7f3b;
      font-weight: 700;
    }

    .result-negative {
      color: #b42318;
      font-weight: 700;
    }

    .loading-placeholder-text {
      display: inline-block;
      min-width: 1.6rem;
      letter-spacing: 0.14em;
      animation: loading-placeholder-pulse 1s ease-in-out infinite;
    }

    @keyframes loading-placeholder-pulse {
      0%, 100% {
        opacity: 0.35;
        transform: translateY(0);
      }

      50% {
        opacity: 1;
        transform: translateY(-1px);
      }
    }

    .portfolio-assets-table tbody tr:nth-child(even) td {
      background: var(--surface-soft);
    }

    @media (max-width: 720px) {
      .portfolio-detail-actions,
      .asset-picker-header,
      .asset-picker-actions,
      .custom-asset-form {
        align-items: stretch;
      }

      .custom-asset-form {
        grid-template-columns: 1fr;
      }

      .portfolio-detail-actions > button,
      .asset-picker-actions > button,
      .custom-asset-form > button {
        width: 100%;
      }
    }
  `],
})
export class InvestmentPortfoliosComponent implements OnInit {
  private readonly fb = inject(FormBuilder);
  private readonly moneyVisibility = inject(MoneyVisibilityService);

  readonly nav = uiMessages.investments.nav;
  readonly messages = uiMessages.investments.portfolios;
  readonly loading = signal(true);
  readonly loadingQuotes = signal(false);
  readonly analysisLoading = signal(false);
  readonly suggestionLoading = signal(false);
  readonly savingPortfolio = signal(false);
  readonly savingAssetChanges = signal(false);
  readonly savingAssetSelection = signal(false);
  readonly reorderingAssets = signal(false);
  readonly panelOpen = signal(false);
  readonly settingsPanelOpen = signal(false);
  readonly watchedCategoryMenuOpen = signal(false);
  readonly editingPortfolioCode = signal<string | null>(null);
  readonly selectedPortfolioCode = signal<string | null>(null);
  readonly portfolioAssetsEditMode = signal(false);
  readonly assetPickerOpen = signal(false);
  readonly customAssetFormOpen = signal(false);
  readonly draggingAssetCode = signal<string | null>(null);
  readonly dropTargetAssetCode = signal<string | null>(null);
  readonly portfolios = signal<InvestmentPortfolio[]>([]);
  readonly assets = signal<InvestmentAsset[]>([]);
  readonly positions = signal<InvestmentPosition[]>([]);
  readonly selectedAnalysis = signal<InvestmentPortfolioAnalysis | null>(null);
  readonly selectedSuggestion = signal<InvestmentPortfolioSuggestion | null>(null);
  readonly suggestionAmount = signal('');
  readonly pendingRebalanceTolerance = signal('0,50');
  readonly pendingSuggestionStrategy = signal<InvestmentSuggestionStrategy>('BEST_NEXT_SHARE');
  readonly pendingWatchedCategoryIDs = signal<string[]>([]);
  readonly assetDrafts = signal<Record<string, PortfolioAssetDraft>>({});
  readonly assetSelections = signal<Record<string, boolean>>({});
  readonly assetSelectionOrder = signal<string[]>([]);
  readonly activeAssets = computed(() => this.assets().filter((asset) => asset.is_active));
  readonly selectedPortfolio = computed(() => this.portfolios().find((portfolio) => portfolio.code === this.selectedPortfolioCode()) ?? null);
  readonly positionByCode = computed(() => new Map(this.positions().map((position) => [position.asset_code, position])));
  readonly availableAssets = computed(() => {
    const selected = this.selectedPortfolio();
    if (!selected) {
      return [];
    }
    const portfolioCodes = new Set(selected.assets.map((asset) => asset.asset_code));
    return this.activeAssets().filter((asset) => !portfolioCodes.has(asset.code));
  });
  readonly selectedAssetCodes = computed(() =>
    this.assetSelectionOrder().filter((code) => this.assetSelections()[code]),
  );
  readonly selectedPortfolioRows = computed<PortfolioDetailRow[]>(() => {
    const portfolio = this.selectedPortfolio();
    if (!portfolio) {
      return [];
    }

    const positions = this.positionByCode();
    const analysisByCode = new Map((this.selectedAnalysis()?.rows ?? []).map((row) => [row.asset_code, row]));
    return portfolio.assets.map((asset) => ({
      asset,
      position: positions.get(asset.asset_code) ?? null,
      analysis: analysisByCode.get(asset.asset_code) ?? null,
    }));
  });
  readonly selectedAnalysisPnlPct = computed(() => {
    const analysis = this.selectedAnalysis();
    const bps = analysis?.total_unrealized_pnl_basis_point;
    return typeof bps === 'number' ? bps / 100 : null;
  });
  readonly selectedAnalysisTargetTotal = computed(() => {
    const analysis = this.selectedAnalysis();
    return analysis ? analysis.target_allocation_basis_point_total / 100 : null;
  });
  readonly selectedAnalysisToleranceLabel = computed(() => {
    const analysis = this.selectedAnalysis();
    return analysis ? this.basisPointsLabel(analysis.rebalance_tolerance_basis_point) : null;
  });
  readonly portfolioForm = this.fb.group({
    name: this.fb.nonNullable.control('', Validators.required),
    description: this.fb.nonNullable.control(''),
  });
  readonly customAssetForm = this.fb.group({
    code: this.fb.nonNullable.control('', Validators.required),
    name: this.fb.nonNullable.control(''),
    assetType: this.fb.nonNullable.control<InvestmentAssetType>('STOCK', Validators.required),
  });

  constructor(
    private readonly investmentsService: InvestmentsService,
    private readonly referenceData: ReferenceDataService,
    private readonly userConfigService: UserConfigService,
    private readonly toast: ToastService,
  ) {}

  ngOnInit(): void {
    this.resetPendingRebalanceTolerance();
    this.load();
  }

  @HostListener('document:click', ['$event'])
  onDocumentClick(event: MouseEvent): void {
    const target = event.target as HTMLElement | null;
    if (target?.closest('[data-multi-select="watched-categories"]')) {
      return;
    }
    this.closeWatchedCategoryMenu();
  }

  load(preferredCode?: string | null): void {
    this.loading.set(true);
    forkJoin({
      config: this.userConfigService.load(),
      referenceData: this.referenceData.load(),
      portfolios: this.investmentsService.listPortfolios(),
      assets: this.investmentsService.listAssets(),
      positions: this.investmentsService.listPositions(),
    }).subscribe({
      next: ({ portfolios, assets, positions }) => {
        this.portfolios.set(portfolios);
        this.assets.set(assets);
        this.positions.set(positions);
        this.resetPendingRebalanceTolerance();
        this.resetPendingSuggestionStrategy();
        this.resetPendingWatchedCategoryIDs();

        const nextSelectedCode = preferredCode ?? this.selectedPortfolioCode();
        const selectedExists = nextSelectedCode ? portfolios.some((portfolio) => portfolio.code === nextSelectedCode) : false;
        this.selectedPortfolioCode.set(selectedExists ? nextSelectedCode : null);
        this.selectedAnalysis.set(null);
        this.selectedSuggestion.set(null);

        if (this.portfolioAssetsEditMode()) {
          this.resetAssetDrafts();
        }
        if (!selectedExists) {
          this.portfolioAssetsEditMode.set(false);
          this.assetPickerOpen.set(false);
          this.customAssetFormOpen.set(false);
          this.assetSelections.set({});
          this.assetSelectionOrder.set([]);
        }

        this.loading.set(false);
        this.loadQuotes();
        this.loadSelectedAnalysis(this.selectedPortfolioCode());
      },
      error: (error) => {
        this.toast.error(getApiErrorMessage(error));
        this.loading.set(false);
      },
    });
  }

  private loadQuotes(): void {
    this.loadingQuotes.set(true);
    this.investmentsService.listPositionQuotes().subscribe({
      next: (quotes) => {
        const byCode = new Map(quotes.map((quote) => [quote.asset_code, quote]));
        this.positions.update((rows) =>
          rows.map((row) => {
            const quote = byCode.get(row.asset_code);
            return quote
              ? { ...row, current_price: quote.current_price, quote_updated_at: quote.quote_updated_at }
              : row;
          }),
        );
      },
      error: (error) => {
        this.toast.error(getApiErrorMessage(error));
        this.loadingQuotes.set(false);
      },
      complete: () => this.loadingQuotes.set(false),
    });
  }

  openCreate(): void {
    this.editingPortfolioCode.set(null);
    this.portfolioForm.reset({ name: '', description: '' });
    this.panelOpen.set(true);
  }

  openEditPortfolio(portfolio: InvestmentPortfolio): void {
    this.editingPortfolioCode.set(portfolio.code);
    this.portfolioForm.reset({
      name: portfolio.name,
      description: portfolio.description,
    });
    this.panelOpen.set(true);
  }

  closePanel(): void {
    this.panelOpen.set(false);
    this.editingPortfolioCode.set(null);
  }

  openSettings(): void {
    this.resetPendingRebalanceTolerance();
    this.resetPendingSuggestionStrategy();
    this.resetPendingWatchedCategoryIDs();
    this.closeWatchedCategoryMenu();
    this.settingsPanelOpen.set(true);
  }

  closeSettings(): void {
    this.closeWatchedCategoryMenu();
    this.settingsPanelOpen.set(false);
  }

  onRebalanceToleranceInput(value: string): void {
    this.pendingRebalanceTolerance.set(this.sanitizeDecimalInput(value));
  }

  formatRebalanceTolerance(): void {
    this.pendingRebalanceTolerance.set(this.formatDecimal(this.pendingRebalanceTolerance(), '0,50'));
  }

  isRebalanceToleranceValid(): boolean {
    const parsed = this.parseLocalizedDecimal(this.pendingRebalanceTolerance());
    return parsed !== null && parsed > 0 && parsed <= 5;
  }

  saveSettings(): void {
    if (!this.isRebalanceToleranceValid()) {
      return;
    }

    const rebalanceToleranceBasisPoint = Math.round(this.parseLocalizedDecimal(this.pendingRebalanceTolerance())! * 100);
    this.userConfigService.updateConfig({
      settings: {
        investments: {
          portfolios: {
            rebalance_tolerance_basis_point: rebalanceToleranceBasisPoint,
            suggestion_strategy: this.pendingSuggestionStrategy(),
          },
          integration: {
            watched_category_ids: this.pendingWatchedCategoryIDs(),
          },
        },
      },
    }).subscribe({
      next: () => {
        this.userConfigService.syncInvestmentPortfoliosConfig({
          rebalance_tolerance_basis_point: rebalanceToleranceBasisPoint,
          suggestion_strategy: this.pendingSuggestionStrategy(),
        });
        this.userConfigService.syncInvestmentIntegrationConfig({
          watched_category_ids: this.pendingWatchedCategoryIDs(),
        });
        this.closeSettings();
        this.loadSelectedAnalysis(this.selectedPortfolioCode());
        if (this.selectedSuggestion()) {
          this.generateSuggestion();
        }
      },
      error: (error) => this.toast.error(getApiErrorMessage(error)),
    });
  }

  toggleWatchedCategoryMenu(): void {
    this.watchedCategoryMenuOpen.update((open) => !open);
  }

  toggleWatchedCategorySelection(categoryID: string): void {
    this.pendingWatchedCategoryIDs.update((ids) => {
      if (ids.includes(categoryID)) {
        return ids.filter((id) => id !== categoryID);
      }
      return [...ids, categoryID];
    });
  }

  isWatchedCategorySelected(categoryID: string): boolean {
    return this.pendingWatchedCategoryIDs().includes(categoryID);
  }

  selectedWatchedCategoriesLabel(): string {
    const selectedIDs = this.pendingWatchedCategoryIDs();
    if (selectedIDs.length === 0) {
      return this.messages.settings.watchedCategoriesPlaceholder;
    }

    const namesByID = new Map(this.watchedIncomeLeafCategories().map((category) => [category.ID, category.Name]));
    return selectedIDs
      .map((id) => namesByID.get(id))
      .filter((name): name is string => Boolean(name))
      .join(', ');
  }

  watchedCategoryGroups(): WatchedCategoryGroup[] {
    const groups: WatchedCategoryGroup[] = [];

    for (const category of this.referenceData.activeCategories()) {
      if (category.Type !== 'INCOME') {
        continue;
      }

      const subCategories = category.SubCategories ?? [];
      if (subCategories.length > 0) {
        const selectableChildren = subCategories.filter((child) => child.Type === 'INCOME' && (child.SubCategories?.length ?? 0) === 0);
        if (selectableChildren.length > 0) {
          groups.push({
            key: category.Code,
            label: category.Name,
            options: selectableChildren,
          });
        }
        continue;
      }

      groups.push({
        key: category.Code,
        label: null,
        options: [category],
      });
    }

    return groups;
  }

  savePortfolio(): void {
    if (this.portfolioForm.invalid) {
      return;
    }

    this.savingPortfolio.set(true);
    const payload = this.portfolioForm.getRawValue();
    const request$ = this.editingPortfolioCode()
      ? this.investmentsService.updatePortfolio(this.editingPortfolioCode()!, payload)
      : this.investmentsService.createPortfolio(payload);

    request$.subscribe({
      next: (portfolio) => {
        this.toast.success('Carteira salva.');
        this.panelOpen.set(false);
        this.editingPortfolioCode.set(null);
        this.selectedPortfolioCode.set(portfolio.code);
        this.load(portfolio.code);
      },
      error: (error) => {
        this.toast.error(getApiErrorMessage(error));
        this.savingPortfolio.set(false);
      },
      complete: () => this.savingPortfolio.set(false),
    });
  }

  togglePortfolioDetails(portfolioCode: string): void {
    const next = this.selectedPortfolioCode() === portfolioCode ? null : portfolioCode;
    this.selectedPortfolioCode.set(next);
    this.selectedAnalysis.set(null);
    this.selectedSuggestion.set(null);
    this.portfolioAssetsEditMode.set(false);
    this.assetPickerOpen.set(false);
    this.customAssetFormOpen.set(false);
    this.assetSelections.set({});
    this.assetSelectionOrder.set([]);
    this.resetAssetDrafts();
    this.loadSelectedAnalysis(next);
  }

  selectPortfolioRow(portfolioCode: string): void {
    this.togglePortfolioDetails(portfolioCode);
  }

  removePortfolio(portfolio: InvestmentPortfolio): void {
    if (!window.confirm(`Excluir a carteira ${portfolio.name}?`)) {
      return;
    }

    this.investmentsService.deletePortfolio(portfolio.code).subscribe({
      next: () => {
        this.toast.success('Carteira excluída.');
        if (this.selectedPortfolioCode() === portfolio.code) {
          this.selectedPortfolioCode.set(null);
          this.portfolioAssetsEditMode.set(false);
          this.assetPickerOpen.set(false);
          this.customAssetFormOpen.set(false);
          this.assetSelections.set({});
          this.assetSelectionOrder.set([]);
        }
        this.load();
      },
      error: (error) => this.toast.error(getApiErrorMessage(error)),
    });
  }

  onAssetDragStart(event: DragEvent, asset: InvestmentPortfolioAsset): void {
    if (this.reorderingAssets() || !this.selectedPortfolio()) {
      event.preventDefault();
      return;
    }

    this.draggingAssetCode.set(asset.asset_code);
    this.dropTargetAssetCode.set(asset.asset_code);
    event.dataTransfer?.setData('text/plain', asset.asset_code);
    if (event.dataTransfer) {
      event.dataTransfer.effectAllowed = 'move';
    }
  }

  onAssetDragOver(event: DragEvent, asset: InvestmentPortfolioAsset): void {
    if (!this.draggingAssetCode() || this.reorderingAssets()) {
      return;
    }

    event.preventDefault();
    this.dropTargetAssetCode.set(asset.asset_code);
    if (event.dataTransfer) {
      event.dataTransfer.dropEffect = 'move';
    }
  }

  onAssetDrop(event: DragEvent, targetAsset: InvestmentPortfolioAsset): void {
    event.preventDefault();

    const sourceCode = this.draggingAssetCode() ?? event.dataTransfer?.getData('text/plain') ?? null;
    this.onAssetDragEnd();

    if (!sourceCode || sourceCode === targetAsset.asset_code || this.reorderingAssets()) {
      return;
    }

    const reorderedPortfolio = this.movePortfolioAsset(sourceCode, targetAsset.asset_code);
    if (!reorderedPortfolio) {
      return;
    }

    this.persistPortfolioAssetReorder(reorderedPortfolio);
  }

  onAssetDragEnd(): void {
    this.draggingAssetCode.set(null);
    this.dropTargetAssetCode.set(null);
  }

  enterAssetEditMode(): void {
    this.portfolioAssetsEditMode.set(true);
    this.assetPickerOpen.set(false);
    this.customAssetFormOpen.set(false);
    this.assetSelections.set({});
    this.assetSelectionOrder.set([]);
    this.resetAssetDrafts();
  }

  cancelAssetEditMode(): void {
    this.portfolioAssetsEditMode.set(false);
    this.assetPickerOpen.set(false);
    this.customAssetFormOpen.set(false);
    this.assetSelections.set({});
    this.assetSelectionOrder.set([]);
    this.resetAssetDrafts();
    this.resetCustomAssetForm();
  }

  toggleAssetPicker(): void {
    const nextOpen = !this.assetPickerOpen();
    this.assetPickerOpen.set(nextOpen);
    if (!nextOpen) {
      this.customAssetFormOpen.set(false);
      this.resetCustomAssetForm();
    }
  }

  toggleCustomAssetForm(): void {
    const nextOpen = !this.customAssetFormOpen();
    this.customAssetFormOpen.set(nextOpen);
    if (!nextOpen) {
      this.resetCustomAssetForm();
    }
  }

  addSelectedAssets(): void {
    const portfolio = this.selectedPortfolio();
    const codes = this.selectedAssetCodes();
    if (!portfolio || codes.length === 0) {
      return;
    }

    this.savingAssetSelection.set(true);
    const nextSortOrder = portfolio.assets.length;
    forkJoin(
      codes.map((assetCode, index) =>
        this.investmentsService.savePortfolioAsset(portfolio.code, assetCode, {
          target_allocation_basis_point: 0,
          max_buy_price: null,
          sort_order: nextSortOrder + index + 1,
        }),
      ),
    ).subscribe({
      next: () => {
        this.toast.success('Ativos adicionados à carteira.');
        this.assetSelections.set({});
        this.assetSelectionOrder.set([]);
        this.assetPickerOpen.set(false);
        this.customAssetFormOpen.set(false);
        this.load(portfolio.code);
      },
      error: (error) => {
        this.toast.error(getApiErrorMessage(error));
        this.savingAssetSelection.set(false);
      },
      complete: () => this.savingAssetSelection.set(false),
    });
  }

  addCustomAsset(): void {
    const portfolio = this.selectedPortfolio();
    if (!portfolio || this.customAssetForm.invalid) {
      return;
    }

    const value = this.customAssetForm.getRawValue();
    const assetCode = value.code.trim().toUpperCase();
    if (!assetCode) {
      this.customAssetForm.controls.code.setErrors({ required: true });
      return;
    }
    const assetName = value.name.trim() || assetCode;
    const existingAsset = this.assets().find((asset) => asset.code === assetCode);
    const alreadyInPortfolio = portfolio.assets.some((asset) => asset.asset_code === assetCode);
    if (alreadyInPortfolio) {
      this.toast.error(this.messages.feedback.assetAlreadyInPortfolio);
      return;
    }

    this.savingAssetSelection.set(true);
    const ensureAsset$ = existingAsset
      ? of(existingAsset)
      : this.investmentsService.createAsset({
          code: assetCode,
          name: assetName,
          asset_type: value.assetType,
        });

    ensureAsset$
      .pipe(
        switchMap((asset) =>
          this.investmentsService.savePortfolioAsset(portfolio.code, asset.code, {
            target_allocation_basis_point: 0,
            max_buy_price: null,
            sort_order: portfolio.assets.length + 1,
          }),
        ),
      )
      .subscribe({
        next: () => {
          this.toast.success(this.messages.feedback.customAssetAdded);
          this.assetPickerOpen.set(false);
          this.customAssetFormOpen.set(false);
          this.resetCustomAssetForm();
          this.load(portfolio.code);
        },
        error: (error) => {
          this.toast.error(getApiErrorMessage(error));
          this.savingAssetSelection.set(false);
        },
        complete: () => this.savingAssetSelection.set(false),
      });
  }

  saveAssetChanges(): void {
    const portfolio = this.selectedPortfolio();
    if (!portfolio || !this.hasValidDrafts()) {
      return;
    }

    this.savingAssetChanges.set(true);
    forkJoin(
      portfolio.assets.map((asset) => {
        const draft = this.assetDraft(asset.asset_code);
        return this.investmentsService.savePortfolioAsset(portfolio.code, asset.asset_code, {
          target_allocation_basis_point: this.parseTargetToBasisPoints(draft.target),
          max_buy_price: draft.maxBuyPrice ? decimalToCents(draft.maxBuyPrice) : null,
          sort_order: asset.sort_order,
        });
      }),
    ).subscribe({
      next: () => {
        this.toast.success('Carteira atualizada.');
        this.portfolioAssetsEditMode.set(false);
        this.assetPickerOpen.set(false);
        this.load(portfolio.code);
      },
      error: (error) => {
        this.toast.error(getApiErrorMessage(error));
        this.savingAssetChanges.set(false);
      },
      complete: () => this.savingAssetChanges.set(false),
    });
  }

  removePortfolioAsset(assetCode: string): void {
    const portfolio = this.selectedPortfolio();
    if (!portfolio || !window.confirm(`Remover ${assetCode} desta carteira?`)) {
      return;
    }

    this.investmentsService.deletePortfolioAsset(portfolio.code, assetCode).subscribe({
      next: () => {
        this.toast.success('Ativo removido da carteira.');
        this.load(portfolio.code);
      },
      error: (error) => this.toast.error(getApiErrorMessage(error)),
    });
  }

  setAssetSelection(assetCode: string, checked: boolean): void {
    this.assetSelections.update((state) => ({ ...state, [assetCode]: checked }));
    this.assetSelectionOrder.update((codes) => {
      if (checked) {
        return codes.includes(assetCode) ? codes : [...codes, assetCode];
      }
      return codes.filter((code) => code !== assetCode);
    });
  }

  isAssetSelected(assetCode: string): boolean {
    return !!this.assetSelections()[assetCode];
  }

  assetDraft(assetCode: string): PortfolioAssetDraft {
    return this.assetDrafts()[assetCode] ?? { target: '0,00', maxBuyPrice: '' };
  }

  onTargetInput(assetCode: string, value: string): void {
    this.patchAssetDraft(assetCode, { target: this.sanitizeDecimalInput(value) });
  }

  formatTarget(assetCode: string): void {
    this.patchAssetDraft(assetCode, { target: this.formatDecimal(this.assetDraft(assetCode).target, '0,00') });
  }

  onMaxBuyPriceInput(assetCode: string, value: string): void {
    this.patchAssetDraft(assetCode, { maxBuyPrice: this.sanitizeDecimalInput(value) });
  }

  formatMaxBuyPrice(assetCode: string): void {
    this.patchAssetDraft(assetCode, { maxBuyPrice: this.formatDecimal(this.assetDraft(assetCode).maxBuyPrice, '') });
  }

  hasValidDrafts(): boolean {
    const portfolio = this.selectedPortfolio();
    if (!portfolio) {
      return false;
    }
    return portfolio.assets.every((asset) => this.isTargetValid(asset.asset_code) && this.isMaxBuyPriceValid(asset.asset_code));
  }

  isTargetValid(assetCode: string): boolean {
    const parsed = this.parseLocalizedDecimal(this.assetDraft(assetCode).target);
    if (parsed === null) {
      return false;
    }
    const bps = Math.round(parsed * 100);
    return bps >= 0 && bps <= 10000;
  }

  isMaxBuyPriceValid(assetCode: string): boolean {
    const raw = this.assetDraft(assetCode).maxBuyPrice.trim();
    if (!raw) {
      return true;
    }
    const parsed = this.parseLocalizedDecimal(raw);
    return parsed !== null && parsed >= 0;
  }

  assetType(type: InvestmentAssetType): string {
    return investmentAssetTypeLabel(type);
  }

  assetLabel(code: string, name: string): string {
    return investmentAssetLabel(code, name);
  }

  money(value: number): string {
    return this.moneyVisibility.formatCurrency(value);
  }

  allocationLabel(value: number): string {
    return `${(value / 100).toFixed(2).replace('.', ',')}%`;
  }

  quoteDisplay(row: PortfolioDetailRow): string {
    if (row.analysis) {
      return this.money(row.analysis.current_price);
    }
    if (row.position && this.loadingQuotes()) {
      return this.messages.states.quotePending;
    }
    return '—';
  }

  averagePriceDisplay(row: PortfolioDetailRow): string {
    const averagePrice = row.analysis?.average_price ?? row.position?.average_price ?? 0;
    if (averagePrice <= 0) {
      return '—';
    }
    return this.money(averagePrice);
  }

  pnlDisplay(row: PortfolioDetailRow): string {
    const bps = row.analysis?.unrealized_pnl_basis_point;
    if (typeof bps !== 'number') {
      return this.analysisLoading() ? this.messages.states.quotePending : '—';
    }
    return this.percentLabel(bps / 100);
  }

  currentAllocationDisplay(row: PortfolioDetailRow): string {
    const bps = row.analysis?.current_allocation_basis_point;
    if (typeof bps !== 'number') {
      return this.analysisLoading() ? this.messages.states.quotePending : '—';
    }
    return this.basisPointsLabel(bps);
  }

  targetAllocationDisplay(row: PortfolioDetailRow): string {
    const bps = row.analysis?.target_allocation_basis_point;
    if (typeof bps === 'number') {
      return this.basisPointsLabel(bps);
    }
    return this.allocationLabel(row.asset.target_allocation_basis_point);
  }

  driftDisplay(row: PortfolioDetailRow): string {
    const bps = row.analysis?.allocation_drift_basis_point;
    if (typeof bps !== 'number') {
      return this.analysisLoading() ? this.messages.states.quotePending : '—';
    }
    return this.signedBasisPointsLabel(bps);
  }

  recommendationLabel(row: PortfolioDetailRow): string {
    if (!row.analysis) {
      return this.analysisLoading() ? this.messages.states.quotePending : '—';
    }
    if (row.analysis.blocked_by_max_buy_price) {
      return this.messages.recommendations.blocked;
    }
    if (row.analysis.allocation_drift_basis_point > 0) {
      return this.messages.recommendations.aboveTarget;
    }
    if (row.analysis.buy_only_gap_amount > 0) {
      return this.messages.recommendations.buy;
    }
    return this.messages.recommendations.balanced;
  }

  suggestionRecommendationLabel(row: InvestmentPortfolioSuggestion['rows'][number]): string {
    if (row.blocked_by_max_buy_price) {
      return this.messages.recommendations.blocked;
    }
    if (row.buy_shares > 0) {
      return this.messages.recommendations.buy;
    }
    if (row.current_allocation_basis_point > row.target_allocation_basis_point) {
      return this.messages.recommendations.aboveTarget;
    }
    return this.messages.recommendations.hold;
  }

  rowPnlPct(row: PortfolioDetailRow): number | null {
    const bps = row.analysis?.unrealized_pnl_basis_point;
    return typeof bps === 'number' ? bps / 100 : null;
  }

  portfolioTotalPnlPercent(portfolio: InvestmentPortfolio): number | null {
    const positions = this.positionByCode();
    let costBasis = 0;
    let currentValue = 0;

    for (const asset of portfolio.assets) {
      const position = positions.get(asset.asset_code);
      if (!position || position.total_cost_basis <= 0 || typeof position.current_price !== 'number') {
        continue;
      }
      costBasis += position.total_cost_basis;
      currentValue += position.current_price * position.current_quantity;
    }

    if (costBasis <= 0) {
      return null;
    }
    return ((currentValue - costBasis) / costBasis) * 100;
  }

  portfolioMatchedDividends(portfolio: InvestmentPortfolio): number {
    const positions = this.positionByCode();
    let dividends = 0;

    for (const asset of portfolio.assets) {
      dividends += positions.get(asset.asset_code)?.matched_dividends_total ?? 0;
    }

    return dividends;
  }

  portfolioTotalPnlDisplay(portfolio: InvestmentPortfolio): string {
    const value = this.portfolioTotalPnlPercent(portfolio);
    if (value === null) {
      return this.loadingQuotes() ? this.messages.states.quotePending : '—';
    }
    return this.percentLabel(value);
  }

  portfolioMatchedDividendsDisplay(portfolio: InvestmentPortfolio): string {
    return this.money(this.portfolioMatchedDividends(portfolio));
  }

  portfolioTotalPnlWithDividendsPercent(portfolio: InvestmentPortfolio): number | null {
    const positions = this.positionByCode();
    let costBasis = 0;
    let currentValue = 0;

    for (const asset of portfolio.assets) {
      const position = positions.get(asset.asset_code);
      if (!position || position.total_cost_basis <= 0 || typeof position.current_price !== 'number') {
        continue;
      }
      costBasis += position.total_cost_basis;
      currentValue += position.current_price * position.current_quantity;
    }

    if (costBasis <= 0) {
      return null;
    }

    return ((currentValue - costBasis + this.portfolioMatchedDividends(portfolio)) / costBasis) * 100;
  }

  portfolioTotalPnlWithDividendsDisplay(portfolio: InvestmentPortfolio): string {
    const value = this.portfolioTotalPnlWithDividendsPercent(portfolio);
    if (value === null) {
      return this.loadingQuotes() ? this.messages.states.quotePending : '—';
    }
    return this.percentLabel(value);
  }

  selectedAnalysisPnlDisplay(): string {
    const value = this.selectedAnalysisPnlPct();
    if (value === null) {
      return this.analysisLoading() ? this.messages.states.quotePending : '—';
    }
    return this.percentLabel(value);
  }

  selectedAnalysisTargetTotalDisplay(): string {
    const value = this.selectedAnalysisTargetTotal();
    if (value === null) {
      return this.analysisLoading() ? this.messages.states.quotePending : '—';
    }
    return this.percentLabel(value);
  }

  minimumSuggestedInvestmentDisplay(): string {
    const value = this.selectedAnalysis()?.minimum_suggested_investment;
    if (typeof value !== 'number') {
      return this.analysisLoading() ? this.messages.states.quotePending : '—';
    }
    return this.money(value);
  }

  matchedDividendsDisplay(): string {
    const value = this.selectedAnalysis()?.income_summary?.matched_dividends_total;
    if (typeof value !== 'number') {
      return this.analysisLoading() ? this.messages.states.quotePending : '—';
    }
    return this.money(value);
  }

  isLoadingLabel(value: string): boolean {
    return value === this.messages.states.quotePending;
  }

  targetAlertLabel(value: number): string {
    return this.messages.detail.targetAlert.replace('{value}', this.percentLabel(value));
  }

  minimumSuggestedInvestmentHelpLabel(): string {
    return this.messages.help.minimumSuggestedInvestment.replace('{value}', this.selectedAnalysisToleranceLabel() ?? '0,00%');
  }

  incomeSummaryHelpLabel(): string {
    const incomeSummary = this.selectedAnalysis()?.income_summary;
    return this.messages.help.incomeSummary
      .replace('{unmatched}', String(incomeSummary?.unmatched_transactions_count ?? 0))
      .replace('{ambiguous}', String(incomeSummary?.ambiguous_transactions_count ?? 0));
  }

  strategyHintLabel(): string {
    return this.pendingSuggestionStrategy() === 'PROPORTIONAL_GAP'
      ? this.messages.settings.strategyHintProportionalGap
      : this.messages.settings.strategyHintBestNextShare;
  }

  selectedAnalysisIncomeRows(): InvestmentPortfolioAnalysis['income_summary']['rows'] {
    return this.selectedAnalysis()?.income_summary?.rows ?? [];
  }

  onSuggestionAmountInput(value: string): void {
    this.suggestionAmount.set(formatAmountDigitsAsCents(value));
  }

  formatSuggestionAmount(): void {
    this.suggestionAmount.set(this.formatDecimal(this.suggestionAmount(), ''));
  }

  isSuggestionAmountValid(): boolean {
    const parsed = this.parseLocalizedDecimal(this.suggestionAmount());
    return parsed !== null && parsed > 0;
  }

  generateSuggestion(): void {
    const portfolio = this.selectedPortfolio();
    if (!portfolio || !this.isSuggestionAmountValid()) {
      return;
    }

    this.suggestionLoading.set(true);
    this.selectedSuggestion.set(null);
    this.investmentsService
      .suggestPortfolioInvestment(portfolio.code, decimalToCents(this.suggestionAmount()))
      .subscribe({
        next: (suggestion) => this.selectedSuggestion.set(suggestion),
        error: (error) => {
          this.toast.error(getApiErrorMessage(error));
          this.suggestionLoading.set(false);
        },
        complete: () => this.suggestionLoading.set(false),
      });
  }

  private persistPortfolioAssetReorder(portfolio: InvestmentPortfolio): void {
    this.reorderingAssets.set(true);
    this.patchPortfolio(portfolio);
    if (this.portfolioAssetsEditMode()) {
      this.resetAssetDrafts();
    }

    this.investmentsService.reorderPortfolioAssets(portfolio.code, portfolio.assets.map((asset) => asset.asset_code)).subscribe({
      next: () => {},
      error: (error) => {
        this.toast.error(getApiErrorMessage(error));
        this.reorderingAssets.set(false);
        this.load(portfolio.code);
      },
      complete: () => this.reorderingAssets.set(false),
    });
  }

  private movePortfolioAsset(sourceCode: string, targetCode: string): InvestmentPortfolio | null {
    const portfolio = this.selectedPortfolio();
    if (!portfolio) {
      return null;
    }

    const assets = [...portfolio.assets];
    const sourceIndex = assets.findIndex((asset) => asset.asset_code === sourceCode);
    const targetIndex = assets.findIndex((asset) => asset.asset_code === targetCode);
    if (sourceIndex === -1 || targetIndex === -1) {
      return null;
    }

    const [movedAsset] = assets.splice(sourceIndex, 1);
    assets.splice(targetIndex, 0, movedAsset);

    return {
      ...portfolio,
      assets: assets.map((asset, index) => ({
        ...asset,
        sort_order: index + 1,
      })),
    };
  }

  private patchPortfolio(nextPortfolio: InvestmentPortfolio): void {
    this.portfolios.update((portfolios) =>
      portfolios.map((portfolio) => (portfolio.code === nextPortfolio.code ? nextPortfolio : portfolio)),
    );
  }

  private resetAssetDrafts(): void {
    const portfolio = this.selectedPortfolio();
    if (!portfolio) {
      this.assetDrafts.set({});
      return;
    }

    const nextDrafts: Record<string, PortfolioAssetDraft> = {};
    for (const asset of portfolio.assets) {
      nextDrafts[asset.asset_code] = {
        target: this.formatBasisPoints(asset.target_allocation_basis_point),
        maxBuyPrice:
          asset.max_buy_price !== null && asset.max_buy_price !== undefined
            ? centsToDecimal(asset.max_buy_price).replace('.', ',')
            : '',
      };
    }
    this.assetDrafts.set(nextDrafts);
  }

  private patchAssetDraft(assetCode: string, patch: Partial<PortfolioAssetDraft>): void {
    this.assetDrafts.update((drafts) => ({
      ...drafts,
      [assetCode]: {
        ...this.assetDraft(assetCode),
        ...patch,
      },
    }));
  }

  private resetCustomAssetForm(): void {
    this.customAssetForm.reset({
      code: '',
      name: '',
      assetType: 'STOCK',
    });
  }

  private sanitizeDecimalInput(value: string): string {
    const cleaned = value.replace(/[^0-9,.\s]/g, '').trim();
    const normalized = cleaned.replace(/\s+/g, '');
    const firstSeparatorIndex = normalized.search(/[,.]/);
    if (firstSeparatorIndex < 0) {
      return normalized;
    }

    const integerPart = normalized.slice(0, firstSeparatorIndex).replace(/[,.]/g, '');
    const decimalPart = normalized.slice(firstSeparatorIndex + 1).replace(/[,.]/g, '');
    return decimalPart ? `${integerPart},${decimalPart}` : `${integerPart},`;
  }

  private formatDecimal(value: string, fallback: string): string {
    const parsed = this.parseLocalizedDecimal(value);
    if (parsed === null) {
      return fallback;
    }
    return new Intl.NumberFormat('pt-BR', {
      minimumFractionDigits: 2,
      maximumFractionDigits: 2,
    }).format(parsed);
  }

  private parseTargetToBasisPoints(value: string): number {
    const parsed = this.parseLocalizedDecimal(value);
    if (parsed === null) {
      return 0;
    }
    return Math.round(parsed * 100);
  }

  private parseLocalizedDecimal(value: string): number | null {
    const trimmed = value.trim();
    if (!trimmed) {
      return null;
    }

    const normalized = trimmed.replace(/\./g, '').replace(',', '.');
    const parsed = Number(normalized);
    if (Number.isNaN(parsed)) {
      return null;
    }
    return parsed;
  }

  private formatBasisPoints(value: number): string {
    return new Intl.NumberFormat('pt-BR', {
      minimumFractionDigits: 2,
      maximumFractionDigits: 2,
    }).format(value / 100);
  }

  private percentLabel(value: number): string {
    return `${value.toFixed(2).replace('.', ',')}%`;
  }

  basisPointsLabel(value: number): string {
    return this.percentLabel(value / 100);
  }

  private resetPendingRebalanceTolerance(): void {
    this.pendingRebalanceTolerance.set(
      this.formatBasisPoints(this.userConfigService.investmentPortfoliosConfig().rebalance_tolerance_basis_point),
    );
  }

  private resetPendingSuggestionStrategy(): void {
    this.pendingSuggestionStrategy.set(this.userConfigService.investmentPortfoliosConfig().suggestion_strategy);
  }

  private resetPendingWatchedCategoryIDs(): void {
    this.pendingWatchedCategoryIDs.set([...this.userConfigService.investmentIntegrationConfig().watched_category_ids]);
  }

  private watchedIncomeLeafCategories(): Category[] {
    return this.referenceData
      .activeFlatCategories()
      .filter((category) => category.Type === 'INCOME' && !(category.SubCategories?.length ?? 0));
  }

  private closeWatchedCategoryMenu(): void {
    this.watchedCategoryMenuOpen.set(false);
  }

  private signedBasisPointsLabel(value: number): string {
    const sign = value > 0 ? '+' : '';
    return `${sign}${this.basisPointsLabel(value)}`;
  }

  private loadSelectedAnalysis(portfolioCode: string | null): void {
    if (!portfolioCode) {
      this.analysisLoading.set(false);
      this.selectedAnalysis.set(null);
      this.selectedSuggestion.set(null);
      return;
    }

    this.analysisLoading.set(true);
    const requestedCode = portfolioCode;
    this.investmentsService.analyzePortfolio(portfolioCode).subscribe({
      next: (analysis) => {
        if (this.selectedPortfolioCode() !== requestedCode) {
          return;
        }
        this.selectedAnalysis.set(analysis);
      },
      error: (error) => {
        if (this.selectedPortfolioCode() !== requestedCode) {
          return;
        }
        this.selectedAnalysis.set(null);
        this.analysisLoading.set(false);
        this.toast.error(getApiErrorMessage(error));
      },
      complete: () => {
        if (this.selectedPortfolioCode() === requestedCode) {
          this.analysisLoading.set(false);
        }
      },
    });
  }
}

function formatAmountDigitsAsCents(value: string): string {
  const digits = value.replace(/\D/g, '');
  if (!digits) {
    return '';
  }

  const integer = digits.slice(0, -2) || '0';
  const cents = digits.slice(-2).padStart(2, '0');
  return `${integer.replace(/^0+(?=\d)/, '') || '0'},${cents}`;
}
