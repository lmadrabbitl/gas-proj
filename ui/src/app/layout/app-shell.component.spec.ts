import { TestBed } from '@angular/core/testing';
import { provideRouter } from '@angular/router';
import { of } from 'rxjs';

import { AuthService } from '../core/auth.service';
import { ReferenceDataService } from '../data/reference-data.service';
import { UserConfigService } from '../data/user-config.service';
import { MoneyVisibilityService } from '../shared/money-visibility.service';
import { AppShellComponent } from './app-shell.component';

describe('AppShellComponent', () => {
  beforeEach(async () => {
    await TestBed.configureTestingModule({
      imports: [AppShellComponent],
      providers: [
        provideRouter([]),
        {
          provide: AuthService,
          useValue: {
            logout: vi.fn(),
          },
        },
        {
          provide: ReferenceDataService,
          useValue: {
            clear: vi.fn(),
          },
        },
        {
          provide: MoneyVisibilityService,
          useValue: {
            hidden: () => false,
            setHidden: vi.fn(),
          },
        },
        {
          provide: UserConfigService,
          useValue: {
            clear: vi.fn(),
            updateUIConfig: vi.fn(() => of({})),
            syncUIConfig: vi.fn(),
          },
        },
      ],
    }).compileComponents();
  });

  it('renders the navigation menu and logout button', () => {
    const fixture = TestBed.createComponent(AppShellComponent);
    fixture.detectChanges();
    const compiled = fixture.nativeElement as HTMLElement;

    expect(compiled.textContent).toContain('Painel');
    expect(compiled.textContent).toContain('Transações');
    expect(compiled.textContent).toContain('Sugestões');
    expect(compiled.textContent).toContain('Sair');
  });

  it('does not expose "null" titles when the sidebar is expanded', () => {
    const fixture = TestBed.createComponent(AppShellComponent);
    fixture.detectChanges();
    const compiled = fixture.nativeElement as HTMLElement;

    const firstNavLink = compiled.querySelector('.nav-list a') as HTMLAnchorElement;
    const logoutButton = compiled.querySelector('.logout-button') as HTMLButtonElement;

    expect(firstNavLink.getAttribute('title')).toBeNull();
    expect(logoutButton.getAttribute('title')).toBeNull();
  });

  it('adds menu and logout titles when the sidebar is collapsed', () => {
    const fixture = TestBed.createComponent(AppShellComponent);
    const component = fixture.componentInstance;

    component.sidebarCollapsed.set(true);
    fixture.detectChanges();

    const compiled = fixture.nativeElement as HTMLElement;
    const firstNavLink = compiled.querySelector('.nav-list a') as HTMLAnchorElement;
    const logoutButton = compiled.querySelector('.logout-button') as HTMLButtonElement;

    expect(firstNavLink.getAttribute('title')).toBe('Painel');
    expect(logoutButton.getAttribute('title')).toBe('Sair');
  });

  it('persists money visibility through user config', () => {
    const fixture = TestBed.createComponent(AppShellComponent);
    const component = fixture.componentInstance;
    const userConfig = TestBed.inject(UserConfigService);

    component.toggleMoneyVisibility();

    expect(userConfig.updateUIConfig).toHaveBeenCalledWith(true);
    expect(userConfig.syncUIConfig).toHaveBeenCalledWith(true);
  });
});
