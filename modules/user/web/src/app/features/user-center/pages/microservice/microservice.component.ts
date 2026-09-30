import { Component, signal } from '@angular/core';
import { TranslateModule } from '@ngx-translate/core';
import { MicroserviceListComponent } from './components/microservice-list.component';
import { CreateEditMicroserviceComponent } from './components/create-edit-microservice.component';
import { MicroserviceTokenListComponent } from './components/microservice-token-list.component';
import { CreateEditMicroserviceTokenComponent } from './components/create-edit-microservice-token.component';
import { MicroserviceTokenFeature } from '../../shared/models/microservice.dto';
import { TabBarComponent, TabItem } from '../../../../shared/components/tab-bar/tab-bar.component';

@Component({
  selector: 'app-microservice',
  standalone: true,
  imports: [
    TranslateModule,
    MicroserviceListComponent,
    CreateEditMicroserviceComponent,
    MicroserviceTokenListComponent,
    CreateEditMicroserviceTokenComponent,
    TabBarComponent
  ],
  templateUrl: './microservice.component.html',
  styleUrl: './microservice.component.css'
})
export class MicroserviceComponent {
  activeTab = signal<'microservices' | 'tokens'>('microservices');
  readonly tabs: TabItem[] = [
    { id: 'microservices', labelKey: 'microservice.microservices' },
    { id: 'tokens', labelKey: 'microservice.tokens' }
  ];

  // Microservice management
  showCreateEditMicroserviceDialog = signal(false);
  editingMicroservice = signal<MicroserviceTokenFeature | null>(null);
  refreshMicroserviceTrigger = signal(0);

  // Token management
  showCreateEditTokenDialog = signal(false);
  refreshTokenTrigger = signal(0);

  onCreateMicroservice(): void {
    this.editingMicroservice.set(null);
    this.showCreateEditMicroserviceDialog.set(true);
  }

  onEditMicroservice(microservice: MicroserviceTokenFeature): void {
    this.editingMicroservice.set(microservice);
    this.showCreateEditMicroserviceDialog.set(true);
  }

  onMicroserviceSaved(): void {
    this.showCreateEditMicroserviceDialog.set(false);
    this.editingMicroservice.set(null);
    this.refreshMicroserviceTrigger.set(this.refreshMicroserviceTrigger() + 1);
  }

  onMicroserviceDialogClose(): void {
    this.showCreateEditMicroserviceDialog.set(false);
    this.editingMicroservice.set(null);
  }

  onCreateToken(): void {
    this.showCreateEditTokenDialog.set(true);
  }

  onTokenSaved(): void {
    this.showCreateEditTokenDialog.set(false);
    this.refreshTokenTrigger.set(this.refreshTokenTrigger() + 1);
  }

  onTokenDialogClose(): void {
    this.showCreateEditTokenDialog.set(false);
  }

  switchTab(tab: 'microservices' | 'tokens'): void {
    this.activeTab.set(tab);
  }
}

