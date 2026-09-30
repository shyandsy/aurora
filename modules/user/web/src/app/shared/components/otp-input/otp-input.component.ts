import {
  Component,
  ElementRef,
  EventEmitter,
  Input,
  Output,
  QueryList,
  ViewChildren,
  forwardRef,
} from '@angular/core';
import { ControlValueAccessor, NG_VALUE_ACCESSOR } from '@angular/forms';

/**
 * 分格验证码输入(6 个数字方框,可复用)。
 *
 * 作为 ControlValueAccessor 直接用 `formControlName` / `[(ngModel)]` 绑定:值 = 拼出的数字串。
 * 交互:输入自动跳下一格、退格回上一格、← →切格、粘贴整串自动分配、纯数字。
 * 仅用于定长纯数字码(TOTP=6 位);恢复码(变长 hex)请走独立文本输入,不用本组件。
 */
@Component({
  selector: 'app-otp-input',
  standalone: true,
  template: `
    <div class="flex justify-center gap-2 sm:gap-3" (paste)="onPaste($event)">
      @for (i of slots; track i) {
      <input
        #box
        type="text"
        inputmode="numeric"
        autocomplete="one-time-code"
        maxlength="1"
        [value]="digits[i]"
        [disabled]="disabled"
        [attr.aria-label]="'Digit ' + (i + 1)"
        (input)="onInput(i, $event)"
        (keydown)="onKeydown(i, $event)"
        (focus)="onFocus($event)"
        class="w-11 h-14 sm:w-12 sm:h-14 text-center text-2xl font-semibold text-gray-900 caret-indigo-500 border border-gray-300 rounded-lg shadow-sm focus:outline-none focus:ring-2 focus:ring-indigo-500 focus:border-indigo-500 disabled:bg-gray-100 disabled:opacity-60 transition"
      />
      }
    </div>
  `,
  providers: [
    {
      provide: NG_VALUE_ACCESSOR,
      useExisting: forwardRef(() => OtpInputComponent),
      multi: true,
    },
  ],
})
export class OtpInputComponent implements ControlValueAccessor {
  /** 方框个数(TOTP 默认 6)。 */
  @Input() length = 6;
  /** 输满 length 位时触发(供调用方自动提交)。 */
  @Output() completed = new EventEmitter<string>();

  @ViewChildren('box') boxes!: QueryList<ElementRef<HTMLInputElement>>;

  digits: string[] = Array.from({ length: 6 }, () => '');
  disabled = false;

  get slots(): number[] {
    return Array.from({ length: this.length }, (_, i) => i);
  }

  private onChange: (v: string) => void = () => {};
  private onTouched: () => void = () => {};

  writeValue(v: string): void {
    const s = (v || '').replace(/\D/g, '').slice(0, this.length).split('');
    this.digits = Array.from({ length: this.length }, (_, i) => s[i] || '');
  }
  registerOnChange(fn: (v: string) => void): void {
    this.onChange = fn;
  }
  registerOnTouched(fn: () => void): void {
    this.onTouched = fn;
  }
  setDisabledState(d: boolean): void {
    this.disabled = d;
  }

  private emit(): void {
    const val = this.digits.join('');
    this.onChange(val);
    if (val.length === this.length) {
      this.completed.emit(val);
    }
  }

  onInput(i: number, e: Event): void {
    const input = e.target as HTMLInputElement;
    const ch = input.value.replace(/\D/g, '').slice(-1); // 只取最后一个数字(防输入法/多字符)
    this.digits[i] = ch;
    input.value = ch;
    this.emit();
    if (ch && i < this.length - 1) {
      this.focusBox(i + 1);
    }
  }

  onKeydown(i: number, e: KeyboardEvent): void {
    if (e.key === 'Backspace') {
      if (this.digits[i]) {
        this.digits[i] = '';
        this.emit();
      } else if (i > 0) {
        this.focusBox(i - 1);
        this.digits[i - 1] = '';
        this.emit();
      }
      e.preventDefault();
    } else if (e.key === 'ArrowLeft' && i > 0) {
      this.focusBox(i - 1);
      e.preventDefault();
    } else if (e.key === 'ArrowRight' && i < this.length - 1) {
      this.focusBox(i + 1);
      e.preventDefault();
    }
  }

  onPaste(e: ClipboardEvent): void {
    const text = (e.clipboardData?.getData('text') || '')
      .replace(/\D/g, '')
      .slice(0, this.length);
    if (!text) {
      return;
    }
    e.preventDefault();
    this.writeValue(text);
    this.emit();
    this.focusBox(Math.min(text.length, this.length - 1));
  }

  onFocus(e: Event): void {
    (e.target as HTMLInputElement).select();
    this.onTouched();
  }

  private focusBox(i: number): void {
    const el = this.boxes?.get(i)?.nativeElement;
    if (el) {
      el.focus();
      el.select();
    }
  }
}
