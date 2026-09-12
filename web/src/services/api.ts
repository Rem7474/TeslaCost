// TeslaCost API Service

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

async function request<T>(endpoint: string, options: RequestInit = {}): Promise<T> {
  const res = await fetch(`${BASE_URL}${endpoint}`, {
    ...options,
    headers: {
      ...getHeaders(),
      ...(options.headers || {}),
    },
  })

  if (res.status === 401) {
    localStorage.removeItem('teslacost_token')
    if (window.location.pathname !== '/login' && window.location.pathname !== '/register') {
      window.location.href = '/login'
    }
  }

  const data = await res.json()
  if (!res.ok) {
    throw new Error(data.error || `Request failed with status ${res.status}`)
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
  syncVehicle: (id: string) => request<any>(`/vehicles/${id}/sync`, { method: 'POST' }),

  // Drives
  getDrives: (vehicleId: string, params?: { tag?: string; page?: number; limit?: number }) => {
    const q = new URLSearchParams()
    if (params?.tag) q.set('tag', params.tag)
    if (params?.page) q.set('page', params.page.toString())
    if (params?.limit) q.set('limit', params.limit.toString())
    return request<any>(`/vehicles/${vehicleId}/drives?${q.toString()}`)
  },
  updateDriveTags: (vehicleId: string, driveId: string, tags: string[]) =>
    request<any>(`/vehicles/${vehicleId}/drives/${driveId}/tags`, { method: 'PATCH', body: JSON.stringify({ tags }) }),
  createTripGroup: (vehicleId: string, payload: { name: string; notes?: string; drive_ids: string[] }) =>
    request<any>(`/vehicles/${vehicleId}/trip-groups`, { method: 'POST', body: JSON.stringify(payload) }),
  getTripGroups: (vehicleId: string) => request<any[]>(`/vehicles/${vehicleId}/trip-groups`),

  // Tires
  getTires: (vehicleId: string) => request<any[]>(`/vehicles/${vehicleId}/tires`),
  createTire: (vehicleId: string, data: any) => request<any>(`/vehicles/${vehicleId}/tires`, { method: 'POST', body: JSON.stringify(data) }),
  addTireLog: (vehicleId: string, tireId: string, data: any) =>
    request<any>(`/vehicles/${vehicleId}/tires/${tireId}/logs`, { method: 'POST', body: JSON.stringify(data) }),
  rotateTires: (vehicleId: string, data: any) =>
    request<any>(`/vehicles/${vehicleId}/tire-rotations`, { method: 'POST', body: JSON.stringify(data) }),

  // Expenses & Charges
  getDriveExpenses: (vehicleId: string) => request<any[]>(`/vehicles/${vehicleId}/expenses`),
  createDriveExpense: (vehicleId: string, data: any) =>
    request<any>(`/vehicles/${vehicleId}/expenses`, { method: 'POST', body: JSON.stringify(data) }),
  getMaintenance: (vehicleId: string) => request<any[]>(`/vehicles/${vehicleId}/maintenance`),
  createMaintenance: (vehicleId: string, data: any) =>
    request<any>(`/vehicles/${vehicleId}/maintenance`, { method: 'POST', body: JSON.stringify(data) }),
  getCharges: (vehicleId: string, page = 1, limit = 50) =>
    request<any>(`/vehicles/${vehicleId}/charges?page=${page}&limit=${limit}`),

  // TCO Analytics
  getTCO: (vehicleId: string) => request<any>(`/vehicles/${vehicleId}/tco`),
}
