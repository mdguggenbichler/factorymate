export type PaginatedResponse<T> = {
  items: T[]
  total: number
}

export type StatusResponse = {
  serverOnline: boolean
  serverName: string
  onlinePlayerCount: number
  trippedCircuits: number[]
  latestMilestone: {
    name: string
    techTier: number
    unlockedAt: string
  } | null
  elevator: {
    name: string
    phaseNumber: number | null
    upgradeReady: boolean
    percentComplete: number | null
  }
}

export type Player = {
  id: string
  name: string
  online: boolean
  lastSeenAt: string | null
}

export type PlayersResponse = {
  players: Player[]
}

export type PlayerHistoryEvent = {
  id: number
  playerId: string
  playerName: string
  eventType: string
  onlineCount: number
  occurredAt: string
}

export type Circuit = {
  circuitId: number
  tripped: boolean
  powerCapacity: number | null
  powerProduction: number | null
  powerConsumed: number | null
  powerMaxConsumed: number | null
  batteryDifferential: number | null
  batteryPercent: number | null
  batteryCapacity: number | null
  batteryTimeEmpty: string | null
  batteryTimeFull: string | null
  updatedAt: string
}

export type PowerResponse = {
  circuits: Circuit[]
}

export type PowerHistoryEvent = {
  id: number
  circuitId: number
  eventType: string
  occurredAt: string
}

export type PowerMetricPoint = {
  id: number
  circuitId: number
  powerProduction: number
  powerConsumed: number
  powerCapacity: number
  batteryPercent: number | null
  capturedAt: string
}

export type PowerMetricsResponse = {
  items: PowerMetricPoint[]
}

export type ProductionItem = {
  itemClassName: string
  itemDisplayName: string
  prodPerMinLabel: string
  prodPercent: number
  consPercent: number
  currentProd: number
  maxProd: number
  currentConsumed: number
  maxConsumed: number
  transferType: string
  updatedAt: string
}

export type ProductionCurrentResponse = {
  items: ProductionItem[]
}

export type ProductionSnapshotPoint = {
  id: number
  itemClassName: string
  itemDisplayName: string
  producedPerMin: number
  consumedPerMin: number
  capturedAt: string
}

export type ProductionHistoryResponse = {
  items: ProductionSnapshotPoint[]
}

export type FactoryItem = {
  name?: string
  className?: string
  amount?: number | string
}

export type ProductionMachine = {
  machineId: string
  buildingType: string
  recipe: string
  manuSpeed: number
  isConfigured: boolean
  isProducing: boolean
  isPaused: boolean
  powerConsumed: number | null
  maxPowerConsumed: number | null
  circuitGroupId: number | null
  ingredients: FactoryItem[]
  production: FactoryItem[]
  updatedAt: string
}

export type ProductionMachinesResponse = {
  machines: ProductionMachine[]
}

export type ResourceSinkResponse = {
  numCoupon: number
  percent: number
  pointsToCoupon: number
  totalPoints: number
  updatedAt: string | null
}

export type ResourceSinkHistoryPoint = {
  id: number
  numCoupon: number
  percent: number
  totalPoints: number
  capturedAt: string
}

export type ResourceSinkHistoryResponse = {
  items: ResourceSinkHistoryPoint[]
}

export type Drone = {
  droneId: string
  homeStation: string | null
  pairedStation: string | null
  hasPairedStation: boolean
  currentDestination: string | null
  flyingSpeed: number | null
  maxSpeed: number | null
  currentFlyingMode: string | null
  updatedAt: string
}

export type DronesResponse = {
  drones: Drone[]
}

export type Doggo = {
  doggoId: string
  name: string | null
  inventory: unknown[]
  updatedAt: string
}

export type DoggosResponse = {
  doggos: Doggo[]
}

export type MilestoneRecipe = {
  name: string
  className: string
}

export type MilestoneSchematic = {
  id: string
  name: string
  purchased: boolean
  locked: boolean
  recipes: MilestoneRecipe[]
  purchasedAt: string | null
  updatedAt: string
}

export type MilestoneGroup = {
  type: string
  techTier: number
  schematics: MilestoneSchematic[]
}

export type MilestonesResponse = {
  groups: MilestoneGroup[]
}

export type ResearchCostItem = {
  name: string
  amount: number
}

export type ResearchCoordinate = {
  x: number
  y: number
}

export type ResearchNode = {
  id: string
  name: string
  category: string | null
  state: string
  techTier: number | null
  cost: ResearchCostItem[]
  coordinates: ResearchCoordinate | null
  parents: ResearchCoordinate[]
  updatedAt: string
}

export type ResearchTree = {
  name: string
  nodes: ResearchNode[]
}

export type ResearchResponse = {
  trees: ResearchTree[]
}

export type Train = {
  trainId: string
  name: string | null
  derailed: boolean
  pendingDerail: boolean
  status: string | null
  selfDrivingError: string | null
  dockingStatus: string | null
  pathStatus: string | null
  station: string | null
  updatedAt: string
}

export type WheeledVehicle = {
  vehicleId: string
  vehicleType: string
  displayName: string
  status: string | null
  driver: string | null
  autopilot: boolean
  followingPath: boolean
  forwardSpeed: number | null
  fuelEmpty: boolean
  lowSpeedSince: string | null
  stuck: boolean
  updatedAt: string
}

export type VehiclesResponse = {
  trains: Train[]
  vehicles: WheeledVehicle[]
}

export type ElevatorPhaseItem = {
  name: string
  className: string
  amount: number
  remainingCost: number
  totalCost: number
}

export type ElevatorResponse = {
  elevatorId: string
  name: string
  upgradeReady: boolean
  phaseNumber: number | null
  currentPhase: ElevatorPhaseItem[]
  updatedAt: string | null
}

export type ElevatorUnknownLogEntry = {
  id: number
  currentPhase: unknown[]
  detectedAt: string
  resolved: boolean
  resolvedAt: string | null
}

export type ElevatorUnknownLogResponse = {
  items: ElevatorUnknownLogEntry[]
}

export type AppSettings = {
  serverName: string
  frmHost: string
  frmPort: number
  frmAuthToken: string
  pollIntervalSeconds: number
  productionSnapshotIntervalSeconds: number
  productionSnapshotRetentionDays: number
}

export type DiscordTargetConfig = {
  channel_id?: string
  thread_id?: string
  webhook_url?: string
  username_override?: string
  avatar_url_override?: string
}

export type DiscordChannel = {
  id: string
  name: string
}

export type DiscordChannelsResponse = {
  channels: DiscordChannel[]
}

export type DiscordSettings = {
  botEnabled: boolean
  botConnected: boolean
  tokenConfigured: boolean
  guildId: string
  roleMappings: RoleMappingsConfig
  autoApprove: boolean
}

export type RoleMappingEntry = {
  discord_role_id: string
  fm_role: "admin" | "viewer"
  bot_commands: string[]
}

export type RoleMappingsConfig = {
  guild_id?: string
  role_mappings: RoleMappingEntry[]
  default_fm_role: "admin" | "viewer"
  default_bot_commands: string[]
  allow_self_register: boolean
  admin_discord_role_ids: string[]
}

export type ConnectionDetails = {
  gameHost: string
  gamePort: number
  gamePassword?: string
  notes?: string
  updatedAt?: string
  updatedByUserId?: number
  smmProfileName?: string
}

export type ModEntry = {
  name: string
  smrName: string
  version: string
  description?: string
  docsUrl?: string
  supportUrl?: string
  createdBy?: string
  remoteVersionRange?: string
  requiredOnRemote: boolean
}

export type ModsResponse = {
  gameBuild: string
  smlVersion: string
  mods: ModEntry[]
  cachedAt: string
  frmReachable: boolean
}

export type PendingRegistration = {
  id: number
  username: string
  pendingPlayerName: string
  externalUsername: string
  externalDisplayName: string
  createdAt: string
}

export type PendingRegistrationsResponse = {
  registrations: PendingRegistration[]
}

export type UnmappedPlayer = {
  playerId: string
  name: string
  online: boolean
  lastSeenAt?: string
}

export type UnmappedPlayersResponse = {
  players: UnmappedPlayer[]
}

export type ExternalIdentity = {
  externalPlatform?: string | null
  externalUserId?: string | null
  externalUsername?: string | null
  externalDisplayName?: string | null
  externalLinkedAt?: string | null
}

export type NotificationTarget = {
  id: number
  name: string
  providerType: string
  config: DiscordTargetConfig
  enabled: boolean
  createdAt: string
}

export type NotificationTargetsResponse = {
  targets: NotificationTarget[]
}

export type EmbedField = {
  name: string
  value: string
  inline: boolean
}

export type EmbedTemplate = {
  title: string
  description: string
  color: string
  fields: EmbedField[]
  footer?: string
  show_timestamp?: boolean
}

export type MessageTemplate = {
  plain?: string
  embed?: EmbedTemplate
}

export type MessageType = {
  key: string
  label: string
  category: string
  enabled: boolean
  variables: string[]
  template: MessageTemplate
  targetIds: number[]
}

export type MessageTypesResponse = {
  messageTypes: MessageType[]
}

export type NotificationLogEntry = {
  id: number
  messageTypeKey: string
  targetId: number
  targetName: string | null
  renderedPreview: string
  success: boolean
  error: string | null
  sentAt: string
}

export type NotificationLogResponse = PaginatedResponse<NotificationLogEntry>

export type AppUser = {
  id: number
  username: string
  role: string
  status: string
  createdAt: string
  playerId?: string | null
  playerName?: string | null
  pendingPlayerName?: string | null
  externalPlatform?: string | null
  externalUserId?: string | null
  externalUsername?: string | null
  externalDisplayName?: string | null
  externalLinkedAt?: string | null
}

export type UsersResponse = {
  users: AppUser[]
}

export type Invite = {
  id: number
  token: string
  role: string
  createdBy: number
  createdAt: string
  expiresAt: string
  status: string
  invitePath: string
  acceptedAt?: string
  acceptedByUserId?: number
  acceptedUsername?: string
  revokedAt?: string
}

export type InvitesResponse = {
  invites: Invite[]
}

export type FRMTestResponse = {
  sessionName: string
  reachable: boolean
}

export type RenderedPreview = {
  plain?: string
  embed?: EmbedTemplate
}
