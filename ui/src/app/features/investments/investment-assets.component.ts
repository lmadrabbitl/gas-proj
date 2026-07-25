import { Component, OnInit, inject, signal } from '@angular/core';
import { FormBuilder, ReactiveFormsModule, Validators } from '@angular/forms';
import { RouterLink, RouterLinkActive } from '@angular/router';

import { InvestmentsService } from '../../data/investments.service';
import { getApiErrorMessage } from '../../shared/api-error';
import { uiMessages } from '../../shared/messages';
import { InvestmentAsset, InvestmentAssetType } from '../../shared/models';
import { ToastService } from '../../shared/toast.service';

@Component({
  selector: 'app-investment-assets',
  imports: [ReactiveFormsModule, RouterLink, RouterLinkActive],
  template: `
    <section class="page-header">
      <div>
        <p class="eyebrow">{{ messages.eyebrow }}</p>
        <h1>{{ messages.title }}</h1>
        <p class="page-subtitle">{{ messages.subtitle }}</p>
      </div>
      <button class="ghost-button" type="button" [disabled]="refreshingMissing()" (click)="refreshMissing()">
        {{ refreshingMissing() ? messages.refreshingMissing : messages.refreshMissing }}
      </button>
    </section>

    <nav class="panel investment-subnav">
      <a routerLink="/investments/dashboard" routerLinkActive="active">{{ nav.dashboard }}</a>
      <a routerLink="/investments/positions" routerLinkActive="active">{{ nav.positions }}</a>
      <a routerLink="/investments/assets" routerLinkActive="active" [routerLinkActiveOptions]="{ exact: true }">
        {{ nav.assets }}
      </a>
      <a routerLink="/investments/insert" routerLinkActive="active">{{ nav.insert }}</a>
      <a routerLink="/investments/operations" routerLinkActive="active">{{ nav.operations }}</a>
      <a routerLink="/investments/portfolios" routerLinkActive="active">{{ nav.portfolios }}</a>
    </nav>

    <section class="panel">
      @if (loading()) {
        <p class="state-message">{{ messages.loading }}</p>
      } @else if (assets().length === 0) {
        <p class="state-message">{{ messages.empty }}</p>
      } @else {
        <div class="table-wrap">
          <table>
            <thead>
              <tr>
                <th>{{ messages.columns.code }}</th>
                <th>{{ messages.columns.name }}</th>
                <th>{{ messages.columns.cnpj }}</th>
                <th></th>
              </tr>
            </thead>
            <tbody>
              @for (asset of assets(); track asset.code) {
                <tr>
                  <td>{{ asset.code }}</td>
                  <td>{{ asset.name }}</td>
                  <td>{{ formatCNPJ(asset.cnpj) }}</td>
                  <td class="actions-cell assets-actions-cell">
                    <button class="icon-action assets-icon-action" type="button" [disabled]="refreshingCode() === asset.code" [title]="messages.actions.refresh" [attr.aria-label]="messages.actions.refresh" (click)="refreshOne(asset)">
                      <svg aria-hidden="true" viewBox="0 0 24 24">
                        <path d="M12 5a7 7 0 0 1 6.31 3.97h-2.56l3.75 3.75 3.75-3.75h-2.58A10 10 0 1 0 22 14h-3a7 7 0 1 1-7-9Z" />
                      </svg>
                    </button>
                    <button class="icon-action assets-icon-action" type="button" [title]="messages.actions.edit" [attr.aria-label]="messages.actions.edit" (click)="openEdit(asset)">
                      <svg aria-hidden="true" viewBox="0 0 24 24">
                        <path
                          d="M4 20h4.5L19 9.5a1.5 1.5 0 0 0 0-2.12l-2.38-2.38A1.5 1.5 0 0 0 14.5 5L4 15.5V20Zm0 0 4.5-4.5M12.5 7.5l4 4"
                          fill="none"
                          stroke="currentColor"
                          stroke-linecap="round"
                          stroke-linejoin="round"
                          stroke-width="1.8"
                        />
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

    @if (editing()) {
      <aside class="side-panel">
        <div class="panel-header">
          <h2>{{ messages.form.title }}</h2>
          <button class="ghost-button" type="button" (click)="closePanel()">{{ messages.actions.close }}</button>
        </div>

        <form class="form-stack" [formGroup]="form" (ngSubmit)="save()">
          <label>
            {{ messages.form.code }}
            <input formControlName="code" />
          </label>
          <label>
            {{ messages.form.name }}
            <input formControlName="name" />
          </label>
          <label>
            {{ messages.form.cnpj }}
            <input formControlName="cnpj" />
          </label>
          <label>
            {{ messages.form.type }}
            <select formControlName="asset_type">
              <option value="STOCK">{{ labels.investmentAssetType.STOCK }}</option>
              <option value="FII">{{ labels.investmentAssetType.FII }}</option>
              <option value="ETF">{{ labels.investmentAssetType.ETF }}</option>
            </select>
          </label>
          <label class="checkbox-row">
            <input type="checkbox" formControlName="is_active" />
            <span>{{ messages.form.active }}</span>
          </label>
          <button class="primary-button" type="submit" [disabled]="saving() || form.invalid">
            {{ saving() ? messages.actions.saving : messages.actions.save }}
          </button>
        </form>
      </aside>
    }
  `,
  styles: [`
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

    .assets-actions-cell {
      width: 1%;
      white-space: nowrap;
    }

    .assets-icon-action {
      width: 24px;
      min-width: 24px;
      min-height: 24px;
      margin-left: 6px;
    }

    .assets-icon-action:disabled {
      opacity: 0.5;
      cursor: default;
    }

    .assets-icon-action:disabled:hover {
      border-color: transparent;
      color: var(--text);
    }

    .checkbox-row {
      display: flex;
      gap: 10px;
      align-items: center;
    }
  `],
})
export class InvestmentAssetsComponent implements OnInit {
  private readonly fb = inject(FormBuilder);
  readonly nav = uiMessages.investments.nav;
  readonly messages = uiMessages.investments.assets;
  readonly labels = uiMessages.labels;
  readonly loading = signal(true);
  readonly saving = signal(false);
  readonly refreshingMissing = signal(false);
  readonly refreshingCode = signal<string | null>(null);
  readonly assets = signal<InvestmentAsset[]>([]);
  readonly editing = signal<InvestmentAsset | null>(null);
  readonly form = this.fb.group({
    code: this.fb.nonNullable.control('', Validators.required),
    name: this.fb.nonNullable.control('', Validators.required),
    cnpj: this.fb.nonNullable.control(''),
    asset_type: this.fb.nonNullable.control<InvestmentAssetType>('STOCK', Validators.required),
    is_active: this.fb.nonNullable.control(true),
  });

  constructor(
    private readonly investmentsService: InvestmentsService,
    private readonly toast: ToastService,
  ) {}

  ngOnInit(): void {
    this.load();
  }

  load(): void {
    this.loading.set(true);
    this.investmentsService.listAssets().subscribe({
      next: (assets) => {
        this.assets.set(assets);
        this.loading.set(false);
      },
      error: (error) => {
        this.toast.error(getApiErrorMessage(error));
        this.loading.set(false);
      },
    });
  }

  openEdit(asset: InvestmentAsset): void {
    this.editing.set(asset);
    this.form.reset({
      code: asset.code,
      name: asset.name,
      cnpj: asset.cnpj ?? '',
      asset_type: asset.asset_type,
      is_active: asset.is_active,
    });
  }

  closePanel(): void {
    this.editing.set(null);
    this.form.reset({
      code: '',
      name: '',
      cnpj: '',
      asset_type: 'STOCK',
      is_active: true,
    });
  }

  save(): void {
    const current = this.editing();
    if (!current || this.form.invalid) {
      this.form.markAllAsTouched();
      return;
    }
    this.saving.set(true);
    const value = this.form.getRawValue();
    this.investmentsService
      .updateAsset(current.code, {
        code: value.code,
        name: value.name,
        cnpj: value.cnpj,
        asset_type: value.asset_type,
        is_active: value.is_active,
      })
      .subscribe({
        next: (asset) => {
          this.assets.update((items) => items.map((item) => (item.code === current.code ? asset : item)));
          this.toast.success('Ativo atualizado.');
          this.saving.set(false);
          this.closePanel();
        },
        error: (error) => {
          this.toast.error(getApiErrorMessage(error));
          this.saving.set(false);
        },
      });
  }

  refreshOne(asset: InvestmentAsset): void {
    this.refreshingCode.set(asset.code);
    this.investmentsService.refreshAssetMetadata(asset.code).subscribe({
      next: (updated) => {
        this.assets.update((items) => items.map((item) => (item.code === asset.code ? updated : item)));
        if (this.editing()?.code === asset.code) {
          this.openEdit(updated);
        }
        this.toast.success('Dados do ativo atualizados.');
        this.refreshingCode.set(null);
      },
      error: (error) => {
        this.toast.error(getApiErrorMessage(error));
        this.refreshingCode.set(null);
      },
    });
  }

  refreshMissing(): void {
    this.refreshingMissing.set(true);
    this.investmentsService.refreshMissingAssetMetadata().subscribe({
      next: (updated) => {
        this.toast.success(updated > 0 ? `${updated} ativo(s) atualizado(s).` : 'Nenhum ativo pendente foi atualizado.');
        this.refreshingMissing.set(false);
        this.load();
      },
      error: (error) => {
        this.toast.error(getApiErrorMessage(error));
        this.refreshingMissing.set(false);
      },
    });
  }

  formatCNPJ(value?: string | null): string {
    const digits = (value ?? '').replace(/\D/g, '');
    if (digits.length !== 14) {
      return value?.trim() || this.messages.states.notAvailable;
    }
    return `${digits.slice(0, 2)}.${digits.slice(2, 5)}.${digits.slice(5, 8)}/${digits.slice(8, 12)}-${digits.slice(12)}`;
  }
}
