import { AccountType, CategoryType, InvestmentAssetType, InvestmentOperationType } from './models';
import { uiMessages } from './messages';

export function accountTypeLabel(type: AccountType): string {
  return uiMessages.labels.accountType[type];
}

export function categoryTypeLabel(type: CategoryType): string {
  return uiMessages.labels.categoryType[type];
}

export function investmentAssetTypeLabel(type: InvestmentAssetType): string {
  return uiMessages.labels.investmentAssetType[type];
}

export function investmentOperationTypeLabel(type: InvestmentOperationType): string {
  return uiMessages.labels.investmentOperationType[type];
}

export function investmentAssetLabel(code: string, name: string): string {
  const normalizedCode = code.trim();
  const normalizedName = name.trim();
  if (!normalizedName || normalizedCode === normalizedName) {
    return normalizedCode;
  }
  return `${normalizedCode} · ${normalizedName}`;
}
