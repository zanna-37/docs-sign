// Placement is the editor's internal representation of a signature on a page. It is
// center-based and measured in PDF points with a top-left origin and clockwise rotation,
// which maps directly to the server's PlacementInput (x = cx - w/2, y = cy - h/2).
export interface Placement {
  id: string;
  signatureId: string;
  page: number; // 0-based
  cx: number;
  cy: number;
  w: number;
  h: number;
  rotation: number; // degrees, clockwise
}

export interface SignatureMeta {
  id: string;
  name: string;
  width: number;
  height: number;
}
