import { Component, signal, inject, OnInit } from '@angular/core';
import { RouterOutlet, Router, NavigationEnd, NavigationError } from '@angular/router';
import { CommonModule } from '@angular/common';
import { filter } from 'rxjs/operators';
import { TopbarComponent } from './shared/components/topbar/topbar.component';
import { ErrorModalComponent } from './shared/components/error-modal/error-modal.component';
import { ErrorService } from './core/services/error.service';
import { StorageService } from './core/services/storage.service';
import { environment } from '../environments/environment';

@Component({
  selector: 'app-root',
  imports: [RouterOutlet, CommonModule, TopbarComponent, ErrorModalComponent],
  templateUrl: './app.html',
  styleUrl: './app.css'
})
export class App implements OnInit {
  protected readonly title = signal('user');
  private readonly router = inject(Router);
  protected readonly errorService = inject(ErrorService);
  private readonly storage = inject(StorageService);

  // 只在本次应用加载后的首个成功导航尝试一次「跳回 returnUrl」,避免每次导航都触发或回环。
  private returnUrlRestored = false;

  // 独立访问 web/user 时的壳 chrome(顶栏:标题/语言/登出)显隐;登录页等门禁壳页不显示。
  // 用户中心是有界工具,只有顶栏 + 页面内顶部 tab 导航,无左侧 sidebar。
  showChrome = signal(false);

  // Expose error service signals for template binding
  get errorMessage() {
    return this.errorService.getErrorMessage();
  }

  get isErrorVisible() {
    return this.errorService.getVisibility();
  }

  onErrorClose() {
    this.errorService.hideError();
  }

  ngOnInit(): void {
    // Check initial route
    this.updateChromeVisibility(this.router.url);

    // Listen to route changes（NavigationEnd = 顶栏显隐;NavigationError = 懒加载 chunk 失败兜底)。
    this.router.events
      .pipe(filter(event => event instanceof NavigationEnd || event instanceof NavigationError))
      .subscribe((event) => {
        if (event instanceof NavigationEnd) {
          this.updateChromeVisibility(event.urlAfterRedirects);
          this.maybeRestoreReturnUrl(event.urlAfterRedirects);
        } else if (event instanceof NavigationError) {
          this.handleNavigationError(event);
        }
      });
  }

  /**
   * 懒加载路由 chunk 加载失败兜底。典型场景:admin 会话过期 → 门禁 cookie(access token 的镜像)失效
   * → 点菜单拉懒加载 chunk 被 traefik 下载门禁挡下(拿到 /gate 登录壳 HTML 而非 JS)→ import() 抛
   * NavigationError。这条走浏览器 module import、**不经 HttpClient**,所以 auth 拦截器的跳登录够不着它;
   * 若不处理,Angular 静默取消导航 → 点菜单像「没反应」(死点)。这里整页跳登录壳(prd/eng=/gate/、
   * dev=/login),让门禁重新校验 + 加载全新 SPA,而非死点。只对「chunk / 动态 import 加载失败」重定向,
   * 避免误伤其它导航错误。
   */
  private handleNavigationError(event: NavigationError): void {
    const err = event.error as { name?: string; message?: string } | undefined;
    const msg = err?.message || String(err ?? '');
    const isChunkFail = err?.name === 'ChunkLoadError'
      || /ChunkLoadError|Loading chunk|dynamically imported module|Failed to fetch dynamically/i.test(msg);
    if (isChunkFail) {
      // 记下用户本要去的那个路由(event.url = 尝试导航的目标),门禁整页往返后跳回,而非一律落首页。
      this.storage.saveReturnUrl(event.url);
      // 整页跳转(非 SPA 内路由)才会重新经过门禁 forwardAuth、落到登录壳并加载全新 SPA。
      window.location.href = environment.loginUrl;
    }
  }

  /**
   * 门禁/登录整页往返后:本次加载的首个成功导航若发现暂存的 returnUrl,就跳回用户原本要去的页。
   * 一次性(returnUrlRestored 兜底),且跳过与当前地址相同 / 空的情况,避免回环与无谓导航。
   */
  private maybeRestoreReturnUrl(currentUrl: string): void {
    if (this.returnUrlRestored) {
      return;
    }
    this.returnUrlRestored = true;
    // 未登录时别急着跳回:那会被守卫又弹回登录页(还白白消费掉 returnUrl)。
    // 此时把 returnUrl 留着,交给登录页在登录成功后消费(见 login.component.applyAndRedirect)。
    // 门禁(prd/eng)整页往返后 SPA 会带着新 token 全新加载,首个导航时已是登录态,走下面跳回。
    if (!this.storage.isAuthenticated()) {
      return;
    }
    const target = this.storage.takeReturnUrl();
    if (target && target !== currentUrl) {
      this.router.navigateByUrl(target);
    }
  }

  private updateChromeVisibility(url: string): void {
    // 顶栏在除登录页外的所有路由显示(登录页自带整屏门禁壳,不套顶栏)。
    this.showChrome.set(!url.includes('/login'));
  }
}
