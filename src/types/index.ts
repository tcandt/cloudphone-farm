// Phone Control Platform — Type Definitions & API Contracts

export type DeviceStatus = 'provisioning' | 'online' | 'offline' | 'degraded' | 'maintenance' | 'revoked' | 'retired';

export interface DeviceCapabilities {
  capture: {
    supported: boolean;
    codecs: string[];
    max_width: number;
    max_height: number;
    max_fps: number;
  };
  control: {
    supported: boolean;
    touch: boolean;
    swipe: boolean;
    global_actions: ('back' | 'home' | 'recents')[];
    text_input: 'none' | 'limited' | 'full';
    sensitive_actions?: boolean; // Reboot, power, install APK - requiring ADB/root
  };
  telemetry: string[];
  transport: string[];
}

export interface DeviceTelemetry {
  battery: number;
  network: 'wifi' | '5g' | '4g' | 'offline';
  orientation?: 0 | 90 | 180 | 270;
  cpu_usage?: number;
  ram_usage?: number;
  temperature_c?: number;
  updated_at: string;
}

export interface DeviceEntity {
  device_id: string;
  organization_id: string;
  display_name: string;
  name?: string;
  model: string;
  android_version: string;
  platform_version?: string;
  serial_number: string;
  status: DeviceStatus;
  capabilities: DeviceCapabilities;
  telemetry: DeviceTelemetry;
  groups: string[];
  group_id?: string | null;
  tags: string[];
  last_seen_at: string;
  active_stream_session_id?: string;
  active_control_lease_id?: string;
  current_operator_name?: string;
}

export interface StreamProfile {
  resolution: '240p' | '360p' | '480p' | '720p';
  fps: number;
  bitrate_kbps: number;
}

export interface StreamSession {
  stream_session_id: string;
  device_id: string;
  organization_id: string;
  user_id: string;
  profile: StreamProfile;
  status: 'requested' | 'signaling' | 'connected' | 'reconnecting' | 'closed' | 'failed';
  started_at: string;
  expires_at: string;
}

export interface ControlLease {
  control_lease_id: string;
  device_id: string;
  organization_id: string;
  user_id: string;
  user_display_name: string;
  fencing_token: number;
  acquired_at: string;
  expires_at: string;
  ttl_seconds: number;
}

export type CommandStatus = 'pending' | 'ack' | 'executing' | 'succeeded' | 'failed';

export type DeviceCommandType =
  | 'gesture.touch'
  | 'gesture.swipe'
  | 'input.text'
  | 'global.back'
  | 'global.home'
  | 'global.recents'
  | 'screen.capture'
  | 'device.reboot'
  | 'device.lock'
  | 'screen.rotate'
  | 'apk.install'
  | 'network.proxy.apply';

export interface DispatchCommandRequest {
  deviceId: string;
  type: DeviceCommandType;
  payload: Record<string, unknown>;
  controlLeaseId?: string;
  idempotencyKey: string;
  issuedAt?: string;
  expiresAt?: string;
}

export interface DeviceCommand {
  command_id: string;
  device_id: string;
  organization_id: string;
  actor_id: string;
  actor_name: string;
  command_type: DeviceCommandType;
  payload: Record<string, unknown>;
  status: CommandStatus;
  error_message?: string;
  created_at: string;
  executed_at?: string;
}

export interface EnrollmentToken {
  token_id: string;
  organization_id: string;
  token_code: string;
  created_by: string;
  expires_at: string;
  used: boolean;
  bound_group_id?: string;
}

export interface AgentEntity {
  agent_id: string;
  device_id: string;
  organization_id: string;
  app_version: string;
  public_key_fingerprint: string;
  status: 'active' | 'disconnected' | 'revoked';
  connected_at: string;
  last_heartbeat_at: string;
}

export type UserRole = 'owner' | 'admin' | 'manager' | 'operator' | 'viewer' | 'billing' | 'support_limited';

export type PermissionCode =
  | 'dashboard.read'
  | 'device.read'
  | 'device.update'
  | 'device.assign'
  | 'device.stream.view'
  | 'device.control.acquire'
  | 'device.control.input'
  | 'device.command.basic'
  | 'device.command.sensitive'
  | 'group.read'
  | 'group.manage'
  | 'agent.read'
  | 'agent.enroll'
  | 'agent.revoke'
  | 'member.read'
  | 'member.invite'
  | 'member.manage'
  | 'role.manage'
  | 'audit.read'
  | 'billing.read'
  | 'billing.manage'
  | 'organization.read'
  | 'organization.manage';

export interface UserSession {
  user_id: string;
  email: string;
  display_name: string;
  organization_id: string;
  organization_name: string;
  role: UserRole;
  permissions: PermissionCode[];
  balance_usd: number;
  avatar_url?: string;
}

export interface DeviceGroup {
  group_id: string;
  organization_id: string;
  name: string;
  description: string;
  color: string;
  device_count: number;
  created_at: string;
}

export interface AuditLogItem {
  audit_id: string;
  organization_id: string;
  action_code: string;
  actor_id: string;
  actor_email: string;
  resource_type: string;
  resource_id: string;
  details: string;
  ip_address: string;
  created_at: string;
}

export interface RentalPackage {
  package_id: string;
  title: string;
  description: string;
  model: string;
  android_version: string;
  ram_storage: string;
  daily_price_usd: number;
  weekly_price_usd: number;
  monthly_price_usd: number;
  available_stock: number;
  badge?: string;
}

export interface AgentKey {
  key_id: string;
  organization_id: string;
  created_by: string;
  name: string;
  token_prefix: string;
  max_bindings?: number;
  active_bindings: number;
  created_at: string;
  updated_at: string;
  expires_at?: string;
  revoked_at?: string;
  last_used_at?: string;
}

export interface AgentKeyBinding {
  binding_id: string;
  device_id: string;
  agent_id: string;
  public_key_fingerprint: string;
  bound_at: string;
  released_at?: string;
  release_reason?: string;
}

export interface AgentKeyCreatedResponse {
  key: AgentKey;
  raw_secret: string;
}
