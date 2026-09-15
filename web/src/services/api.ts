// TeslaCost API Service
import { newIdempotencyKey } from '@/services/offlineQueue'

const BASE_URL = '/api'

function getHeaders(): HeadersInit {
  const token = localStorage.getItem('teslacost_token')
  const headers: HeadersInit = {
    'Content-Type': 'application/json',
  }
  if (token) {
    headers['Authorization'] = `Bearer ${token}`
  }
  return headers
}

export interface QueuedResult {
  queued: true
}

async function request<T>(endpoint: string, options: RequestInit = {}, offlineLabel?: string): Promise<T> {
  const idempotencyKey = offlineLabel ? newIdempotencyKey() : undefined
  const queueForLater = async () => {
    const { useOfflineStore } = await import('@/stores/offline')
    await useOfflineStore().queue({
      id: idempotencyKey!,
      method: options.method || 'POST',
      endpoint,
      body: typeof options.body === 'string' ? options.body : undefined,
      label: offlineLabel!,
      createdAt: Date.now(),
    })
    return { queued: true } as T
  }

  if (offlineLabel && !navigator.onLine) {
    return queueForLater()
  }

  let res: Response
  try {
    res = await fetch(`${BASE_URL}${endpoint}`, {
      ...options,
      headers: {
        ...getHeaders(),
        ...(idempotencyKey ? { 'Idempotency-Key': idempotencyKey } : {}),
        ...(options.headers || {}),
      },
    })
  } catch (err) {
    // Network failure: mutations flagged for offline use are kept and replayed later
    if (offlineLabel && err instanceof TypeError) {
      return queueForLater()
    }
    throw err
  }

  if (res.status === 401) {
    localStorage.removeItem('teslacost_token')
    if (window.location.pathname !== '/login' && window.location.pathname !== '/register' && window.location.pathname !== '/onboarding') {
      window.location.href = '/login'
    }
  }

  let data: any
  const contentType = res.headers.get('content-type') || ''
  if (contentType.includes('application/json')) {
    data = await res.json()
  } else {
    throw new Error(
      res.ok
        ? 'Réponse du serveur invalide'
        : `Erreur (${res.status}): La base de données ou le service n'est pas prêt`
    )
  }

  if (!res.ok) {
    throw new Error(data.error || `La requête a échoué (${res.status})`)
  }

  return data as T
}

export const api = {
  // Auth
  login: (credentials: any) => request<any>('/auth/login', { method: 'POST', body: JSON.stringify(credentials) }),
  register: (payload: any) => request<any>('/auth/register', { method: 'POST', body: JSON.stringify(payload) }),
  getMe: () => request<any>('/auth/me'),

  // Vehicles
  getVehicles: () => request<any[]>('/vehicles'),
  createVehicle: (data: any) => request<any>('/vehicles', { method: 'POST', body: JSON.stringify(data) }),
  updateVehicle: (id: string, data: any) => request<any>(`/vehicles/${id}`, { method: 'PUT', body: JSON.stringify(data) }),
  deleteVehicle: (id: string) => request<any>(`/vehicles/${id}`, { method: 'DELETE' }),
  testTeslaMate: (id: string) => request<any>(`/vehicles/${id}/teslamate/test`, { method: 'POST' }),
  testTeslaMateRaw: (payload: any) =>
    request<any>('/vehicles/test-connection', { method: 'POST', body: JSON.stringify(payload) }),
  syncVehicle: (id: string) => request<any>(`/vehicles/${id}/sync`, { method: 'POST' }),
  getSyncStatus: (id: string) => request<any>(`/vehicles/${id}/sync`),
  getOwnership: (id: string) => request<any>(`/vehicles/${id}/ownership`),
  saveOwnership: (id: string, data: any) =>
    request<any>(`/vehicles/${id}/ownership`, { method: 'PUT', body: JSON.stringify(data) }),
  deleteOwnership: (id: string) => request<any>(`/vehicles/${id}/ownership`, { method: 'DELETE' }),
  updatePreTeslaMateEnergy: (id: string, data: { pre_teslamate_kwh_100km?: number | null; pre_teslamate_eur_per_kwh?: number | null }) =>
    request<any>(`/vehicles/${id}/pre-teslamate-energy`, { method: 'PUT', body: JSON.stringify(data) }),
  getOdometerAt: (vehicleId: string, date: string) =>
    request<{ odometer: number; source: string }>(`/vehicles/${vehicleId}/odometer-at?date=${encodeURIComponent(date)}`),
  getDataQuality: (vehicleId: string) => request<any>(`/vehicles/${vehicleId}/data-quality`),

  // Odometer Checkpoints
  getOdometerCheckpoints: (vehicleId: string) => request<any[]>(`/vehicles/${vehicleId}/odometer-checkpoints`),
  createOdometerCheckpoint: (vehicleId: string, data: any) =>
    request<any>(`/vehicles/${vehicleId}/odometer-checkpoints`, { method: 'POST', body: JSON.stringify(data) }),
  updateOdometerCheckpoint: (vehicleId: string, checkpointId: string, data: any) =>
    request<any>(`/vehicles/${vehicleId}/odometer-checkpoints/${checkpointId}`, { method: 'PUT', body: JSON.stringify(data) }),
  deleteOdometerCheckpoint: (vehicleId: string, checkpointId: string) =>
    request<any>(`/vehicles/${vehicleId}/odometer-checkpoints/${checkpointId}`, { method: 'DELETE' }),

  // Drives
  getDrives: (vehicleId: string, params?: { tag?: string; page?: number; limit?: number; unqualified?: boolean; tripGroupId?: string }) => {
    const q = new URLSearchParams()
    if (params?.tag) q.set('tag', params.tag)
    if (params?.tripGroupId) q.set('trip_group_id', params.tripGroupId)
    if (params?.unqualified) q.set('unqualified', 'true')
    if (params?.page) q.set('page', params.page.toString())
    if (params?.limit) q.set('limit', params.limit.toString())
    return request<any>(`/vehicles/${vehicleId}/drives?${q.toString()}`)
  },
  updateDriveTags: (vehicleId: string, driveId: string, tags: string[]) =>
    request<any>(`/vehicles/${vehicleId}/drives/${driveId}/tags`, { method: 'PATCH', body: JSON.stringify({ tags }) }),
  setDriveTollReview: (vehicleId: string, driveId: string, reviewed: boolean) =>
    request<any>(
      `/vehicles/${vehicleId}/drives/${driveId}/toll-review`,
      { method: 'PATCH', body: JSON.stringify({ reviewed }) },
      'Trajet marqué sans péage'
    ),
  createTripGroup: (vehicleId: string, payload: { name: string; notes?: string; drive_ids: string[] }) =>
    request<any>(`/vehicles/${vehicleId}/trip-groups`, { method: 'POST', body: JSON.stringify(payload) }),
  getTripGroups: (vehicleId: string) => request<any[]>(`/vehicles/${vehicleId}/trip-groups`),
  updateTripGroup: (vehicleId: string, groupId: string, payload: { name: string; notes?: string | null; drive_ids?: string[] }) =>
    request<any>(`/vehicles/${vehicleId}/trip-groups/${groupId}`, { method: 'PUT', body: JSON.stringify(payload) }),
  deleteTripGroup: (vehicleId: string, groupId: string, deleteExpenses = false) =>
    request<any>(`/vehicles/${vehicleId}/trip-groups/${groupId}?delete_expenses=${deleteExpenses}`, { method: 'DELETE' }),

  // Tires
  getTires: (vehicleId: string) => request<any[]>(`/vehicles/${vehicleId}/tires`),
  createTire: (vehicleId: string, data: any) => request<any>(`/vehicles/${vehicleId}/tires`, { method: 'POST', body: JSON.stringify(data) }),
  batchCreateTires: (vehicleId: string, data: any) =>
    request<any>(`/vehicles/${vehicleId}/tires/batch`, { method: 'POST', body: JSON.stringify(data) }),
  updateTire: (vehicleId: string, tireId: string, data: any) =>
    request<any>(`/vehicles/${vehicleId}/tires/${tireId}`, { method: 'PUT', body: JSON.stringify(data) }),
  quickRotateTires: (vehicleId: string, data: { mode: string; odometer: number; swap_with_pack_tire_ids?: string[] }) =>
    request<any>(`/vehicles/${vehicleId}/tires/quick-rotate`, { method: 'POST', body: JSON.stringify(data) }),
  getTireHistory: (vehicleId: string, tireId: string) =>
    request<any>(`/vehicles/${vehicleId}/tires/${tireId}/history`),
  createTireSession: (vehicleId: string, tireId: string, data: any) =>
    request<any>(`/vehicles/${vehicleId}/tires/${tireId}/sessions`, { method: 'POST', body: JSON.stringify(data) }),
  updateTireSession: (vehicleId: string, tireId: string, sessionId: string, data: any) =>
    request<any>(`/vehicles/${vehicleId}/tires/${tireId}/sessions/${sessionId}`, { method: 'PUT', body: JSON.stringify(data) }),
  deleteTireSession: (vehicleId: string, tireId: string, sessionId: string) =>
    request<any>(`/vehicles/${vehicleId}/tires/${tireId}/sessions/${sessionId}`, { method: 'DELETE' }),
  batchUpdateTires: (vehicleId: string, data: any) =>
    request<any>(`/vehicles/${vehicleId}/tires/batch`, { method: 'PATCH', body: JSON.stringify(data) }),
  deleteTire: (vehicleId: string, tireId: string) =>
    request<any>(`/vehicles/${vehicleId}/tires/${tireId}`, { method: 'DELETE' }),
  disposeTire: (vehicleId: string, tireId: string, data: { date: string; odometer?: number | null }) =>
    request<any>(`/vehicles/${vehicleId}/tires/${tireId}/dispose`, { method: 'POST', body: JSON.stringify(data) }),
  updateTireLog: (vehicleId: string, tireId: string, logId: string, data: any) =>
    request<any>(`/vehicles/${vehicleId}/tires/${tireId}/logs/${logId}`, { method: 'PUT', body: JSON.stringify(data) }),
  deleteTireLog: (vehicleId: string, tireId: string, logId: string) =>
    request<any>(`/vehicles/${vehicleId}/tires/${tireId}/logs/${logId}`, { method: 'DELETE' }),
  addTireLog: (vehicleId: string, tireId: string, data: any) =>
    request<any>(`/vehicles/${vehicleId}/tires/${tireId}/logs`, { method: 'POST', body: JSON.stringify(data) }),
  rotateTires: (vehicleId: string, data: any) =>
    request<any>(`/vehicles/${vehicleId}/tire-rotations`, { method: 'POST', body: JSON.stringify(data) }),

  // Expenses & Charges
  getDriveExpenses: (vehicleId: string) => request<any[]>(`/vehicles/${vehicleId}/expenses`),
  createDriveExpense: (vehicleId: string, data: any) =>
    request<any>(`/vehicles/${vehicleId}/expenses`, { method: 'POST', body: JSON.stringify(data) }, `Péage / parking de ${data.amount} ${data.currency || 'EUR'}`),
  updateDriveExpense: (vehicleId: string, expenseId: string, data: any) =>
    request<any>(`/vehicles/${vehicleId}/expenses/${expenseId}`, { method: 'PUT', body: JSON.stringify(data) }, 'Modification de péage / parking'),
  deleteDriveExpense: (vehicleId: string, expenseId: string) =>
    request<void>(`/vehicles/${vehicleId}/expenses/${expenseId}`, { method: 'DELETE' }),
  getMaintenance: (vehicleId: string) => request<any[]>(`/vehicles/${vehicleId}/maintenance`),
  createMaintenance: (vehicleId: string, data: any) =>
    request<any>(`/vehicles/${vehicleId}/maintenance`, { method: 'POST', body: JSON.stringify(data) }, `Dépense « ${data.description} »`),
  updateMaintenance: (vehicleId: string, maintenanceId: string, data: any) =>
    request<any>(`/vehicles/${vehicleId}/maintenance/${maintenanceId}`, { method: 'PUT', body: JSON.stringify(data) }),
  deleteMaintenance: (vehicleId: string, maintenanceId: string) =>
    request<void>(`/vehicles/${vehicleId}/maintenance/${maintenanceId}`, { method: 'DELETE' }),
  getCharges: (vehicleId: string, params: { page?: number; limit?: number; missingCost?: boolean } = {}) => {
    const q = new URLSearchParams({ page: String(params.page || 1), limit: String(params.limit || 50) })
    if (params.missingCost) q.set('missing_cost', 'true')
    return request<any>(`/vehicles/${vehicleId}/charges?${q.toString()}`)
  },
  createCharge: (vehicleId: string, data: any) =>
    request<any>(`/vehicles/${vehicleId}/charges`, { method: 'POST', body: JSON.stringify(data) }, `Recharge de ${data.kwh_added} kWh`),
  updateCharge: (vehicleId: string, chargeId: string, data: any) =>
    request<any>(`/vehicles/${vehicleId}/charges/${chargeId}`, { method: 'PUT', body: JSON.stringify(data) }, 'Coût de recharge'),
  deleteCharge: (vehicleId: string, chargeId: string) =>
    request<any>(`/vehicles/${vehicleId}/charges/${chargeId}`, { method: 'DELETE' }),

  getDriveExpensesForDrive: (vehicleId: string, driveId: string) =>
    request<any[]>(`/vehicles/${vehicleId}/drives/${driveId}/expenses`),

  // TCO Analytics
  getTCO: (vehicleId: string) => request<any>(`/vehicles/${vehicleId}/tco`),

  // Carpooling / BlaBlaCar
  getCarpools: (vehicleId: string) => request<{ trips: any[]; summary: any }>(`/vehicles/${vehicleId}/carpools`),
  getCarpool: (vehicleId: string, id: string) => request<any>(`/vehicles/${vehicleId}/carpools/${id}`),
  createCarpool: (vehicleId: string, data: any) =>
    request<any>(`/vehicles/${vehicleId}/carpools`, { method: 'POST', body: JSON.stringify(data) }),
  updateCarpool: (vehicleId: string, id: string, data: any) =>
    request<any>(`/vehicles/${vehicleId}/carpools/${id}`, { method: 'PUT', body: JSON.stringify(data) }),
  deleteCarpool: (vehicleId: string, id: string) =>
    request<any>(`/vehicles/${vehicleId}/carpools/${id}`, { method: 'DELETE' }),
  estimateCarpoolCosts: (vehicleId: string, params: { drive_id?: string; trip_group_id?: string; drive_ids?: string[]; distance_km?: number }) => {
    const q = new URLSearchParams()
    if (params.drive_id) q.set('drive_id', params.drive_id)
    if (params.trip_group_id) q.set('trip_group_id', params.trip_group_id)
    if (params.drive_ids && params.drive_ids.length > 0) q.set('drive_ids', params.drive_ids.join(','))
    if (params.distance_km !== undefined) q.set('distance_km', params.distance_km.toString())
    return request<any>(`/vehicles/${vehicleId}/carpools/estimate?${q.toString()}`)
  },
}
