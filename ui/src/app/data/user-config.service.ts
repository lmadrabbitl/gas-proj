import { HttpClient } from '@angular/common/http';
import { Injectable, computed, signal } from '@angular/core';
import { map, of, tap } from 'rxjs';

import { MoneyVisibilityService } from '../shared/money-visibility.service';
import { setApiMessageLanguage } from '../shared/api-messages';
import {
  InvestmentIntegrationConfig,
  InvestmentPortfoliosConfig,
  InvestmentSuggestionStrategy,
  ReportsConfig,
  TransactionListConfig,
  UserConfig,
} from '../shared/models';

const DEFAULT_LANGUAGE = 'pt-BR';
const DEFAULT_TRANSACTION_LIST_CONFIG: TransactionListConfig = {
  page_size: 50,
  show_total: false,
};
const DEFAULT_REPORTS_CONFIG: ReportsConfig = {
  show_empty_categories: true,
};
const DEFAULT_INVESTMENT_PORTFOLIOS_CONFIG: InvestmentPortfoliosConfig = {
  rebalance_tolerance_basis_point: 50,
  suggestion_strategy: 'BEST_NEXT_SHARE',
};
const DEFAULT_INVESTMENT_INTEGRATION_CONFIG: InvestmentIntegrationConfig = {
  watched_category_ids: [],
};
const DEFAULT_HIDE_AMOUNTS = false;

type UpdateUserConfigPayload = Partial<{
  language: string;
  settings: Partial<{
    transactions: {
      list: Partial<TransactionListConfig>;
    };
    reports: Partial<ReportsConfig>;
    investments: Partial<{
      portfolios: Partial<InvestmentPortfoliosConfig>;
      integration: Partial<InvestmentIntegrationConfig>;
    }>;
    ui: {
      hide_amounts: boolean;
    };
  }>;
}>;

@Injectable({ providedIn: 'root' })
export class UserConfigService {
  private readonly loadedSignal = signal(false);
  readonly config = signal<UserConfig>(this.defaultConfig());
  readonly transactionListConfig = computed(() => this.config().settings.transactions.list);
  readonly reportsConfig = computed(() => this.config().settings.reports);
  readonly investmentPortfoliosConfig = computed(() => this.config().settings.investments.portfolios);
  readonly investmentIntegrationConfig = computed(() => this.config().settings.investments.integration);
  readonly hideAmounts = computed(() => this.config().settings.ui.hide_amounts);

  constructor(
    private readonly http: HttpClient,
    private readonly moneyVisibility: MoneyVisibilityService,
  ) {
    setApiMessageLanguage(this.config().language);
  }

  load() {
    if (this.loadedSignal()) {
      return of(this.config());
    }
    return this.reload();
  }

  reload() {
    return this.http.get<{ config: UserConfig }>('/api/users/me/config').pipe(
      map((response) => response.config),
      tap((config) => this.applyConfig(config)),
    );
  }

  updateConfig(payload: UpdateUserConfigPayload) {
    return this.http.patch<{ config: UserConfig }>('/api/users/me/config', payload).pipe(
      map((response) => response.config),
      tap((config) => this.applyConfig(config)),
    );
  }

  updateTransactionListConfig(config: TransactionListConfig) {
    return this.updateConfig({
      settings: {
        transactions: {
          list: config,
        },
      },
    });
  }

  syncTransactionListConfig(config: TransactionListConfig): void {
    this.applyConfig({
      ...this.config(),
      settings: {
        ...this.config().settings,
        transactions: {
          ...this.config().settings.transactions,
          list: {
            page_size: config.page_size,
            show_total: config.show_total,
          },
        },
      },
    });
  }

  updateReportsConfig(config: ReportsConfig) {
    return this.updateConfig({
      settings: {
        reports: config,
      },
    });
  }

  syncReportsConfig(config: ReportsConfig): void {
    this.applyConfig({
      ...this.config(),
      settings: {
        ...this.config().settings,
        reports: {
          show_empty_categories: config.show_empty_categories,
        },
      },
    });
  }

  updateUIConfig(hideAmounts: boolean) {
    return this.updateConfig({
      settings: {
        ui: {
          hide_amounts: hideAmounts,
        },
      },
    });
  }

  syncUIConfig(hideAmounts: boolean): void {
    this.applyConfig({
      ...this.config(),
      settings: {
        ...this.config().settings,
        ui: {
          hide_amounts: hideAmounts,
        },
      },
    });
  }

  updateInvestmentPortfoliosConfig(config: InvestmentPortfoliosConfig) {
    return this.updateConfig({
      settings: {
        investments: {
          portfolios: config,
        },
      },
    });
  }

  updateInvestmentIntegrationConfig(config: InvestmentIntegrationConfig) {
    return this.updateConfig({
      settings: {
        investments: {
          integration: config,
        },
      },
    });
  }

  syncInvestmentPortfoliosConfig(config: InvestmentPortfoliosConfig): void {
    this.applyConfig({
      ...this.config(),
      settings: {
        ...this.config().settings,
        investments: {
          ...this.config().settings.investments,
          portfolios: {
            rebalance_tolerance_basis_point: config.rebalance_tolerance_basis_point,
            suggestion_strategy: config.suggestion_strategy,
          },
        },
      },
    });
  }

  syncInvestmentIntegrationConfig(config: InvestmentIntegrationConfig): void {
    this.applyConfig({
      ...this.config(),
      settings: {
        ...this.config().settings,
        investments: {
          ...this.config().settings.investments,
          integration: {
            watched_category_ids: [...config.watched_category_ids],
          },
        },
      },
    });
  }

  clear(): void {
    this.loadedSignal.set(false);
    this.config.set(this.defaultConfig());
    setApiMessageLanguage(DEFAULT_LANGUAGE);
    this.moneyVisibility.setHidden(DEFAULT_HIDE_AMOUNTS);
  }

  private applyConfig(config: UserConfig): void {
    const nextConfig: UserConfig = {
      language: config.language || DEFAULT_LANGUAGE,
      settings: {
        transactions: {
          list: {
            page_size: config.settings?.transactions?.list?.page_size ?? DEFAULT_TRANSACTION_LIST_CONFIG.page_size,
            show_total: config.settings?.transactions?.list?.show_total ?? DEFAULT_TRANSACTION_LIST_CONFIG.show_total,
          },
        },
        reports: {
          show_empty_categories: config.settings?.reports?.show_empty_categories ?? DEFAULT_REPORTS_CONFIG.show_empty_categories,
        },
        investments: {
          portfolios: {
            rebalance_tolerance_basis_point:
              config.settings?.investments?.portfolios?.rebalance_tolerance_basis_point ??
              DEFAULT_INVESTMENT_PORTFOLIOS_CONFIG.rebalance_tolerance_basis_point,
            suggestion_strategy: this.normalizeInvestmentSuggestionStrategy(
              config.settings?.investments?.portfolios?.suggestion_strategy,
            ),
          },
          integration: {
            watched_category_ids:
              config.settings?.investments?.integration?.watched_category_ids?.filter((id) => !!id) ??
              DEFAULT_INVESTMENT_INTEGRATION_CONFIG.watched_category_ids,
          },
        },
        ui: {
          hide_amounts: config.settings?.ui?.hide_amounts ?? DEFAULT_HIDE_AMOUNTS,
        },
      },
    };
    this.config.set(nextConfig);
    this.loadedSignal.set(true);
    setApiMessageLanguage(nextConfig.language);
    this.moneyVisibility.setHidden(nextConfig.settings.ui.hide_amounts);
  }

  private defaultConfig(): UserConfig {
    return {
      language: DEFAULT_LANGUAGE,
      settings: {
        transactions: {
          list: { ...DEFAULT_TRANSACTION_LIST_CONFIG },
        },
        reports: { ...DEFAULT_REPORTS_CONFIG },
        investments: {
          portfolios: { ...DEFAULT_INVESTMENT_PORTFOLIOS_CONFIG },
          integration: { ...DEFAULT_INVESTMENT_INTEGRATION_CONFIG },
        },
        ui: {
          hide_amounts: DEFAULT_HIDE_AMOUNTS,
        },
      },
    };
  }

  private normalizeInvestmentSuggestionStrategy(
    strategy?: InvestmentSuggestionStrategy | null,
  ): InvestmentSuggestionStrategy {
    return strategy === 'PROPORTIONAL_GAP' ? strategy : DEFAULT_INVESTMENT_PORTFOLIOS_CONFIG.suggestion_strategy;
  }
}
