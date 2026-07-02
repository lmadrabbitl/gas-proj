import { inject } from '@angular/core';
import { CanActivateFn, Router } from '@angular/router';
import { catchError, forkJoin, map, of } from 'rxjs';

import { AuthService } from './auth.service';
import { ReferenceDataService } from '../data/reference-data.service';
import { ToastService } from '../shared/toast.service';
import { UserConfigService } from '../data/user-config.service';

export const authGuard: CanActivateFn = () => {
  const auth = inject(AuthService);
  const router = inject(Router);
  const referenceData = inject(ReferenceDataService);
  const toast = inject(ToastService);
  const userConfig = inject(UserConfigService);

  if (auth.isLoggedIn()) {
    return forkJoin([referenceData.load(), userConfig.load()]).pipe(
      map(() => true),
      catchError(() => {
        auth.logout();
        referenceData.clear();
        toast.clear();
        userConfig.clear();
        return of(router.createUrlTree(['/login']));
      }),
    );
  }

  return router.createUrlTree(['/login']);
};
