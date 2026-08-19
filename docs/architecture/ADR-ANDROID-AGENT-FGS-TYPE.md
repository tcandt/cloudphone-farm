# ADR: ANDROID AGENT FOREGROUND SERVICE TYPE SELECTION

**Date:** 2026-08-19  
**Status:** PROPOSED (To be validated in Phase 4)  
**Context:** CloudPhoneRental Agent V2  

## 1. Context and Problem Statement
Starting from Android 14 (API level 34), the Android operating system requires that every Foreground Service (FGS) explicitly declare its purpose using the `android:foregroundServiceType` attribute in the `AndroidManifest.xml`. The OS imposes strict prerequisites and runtime restrictions for starting each type of FGS, particularly from background contexts like `BOOT_COMPLETED`.

Our `AgentConnectionService` is responsible for maintaining a persistent WebSocket connection (WSS) to the Backend cluster and processing control commands. We need to determine the correct and most resilient FGS type for this service that complies with Android 14+ policies while guaranteeing recovery attempts after device reboots.

## 2. Considered Options

We are evaluating two primary options for the FGS type of `AgentConnectionService`:

### Option A: `connectedDevice`
- **Purpose:** Designed for services that interact with external hardware devices (like Bluetooth, USB, or certain network peripherals).
- **Manifest Declaration:** `android:foregroundServiceType="connectedDevice"`
- **Android API Requirements:** API 34+.
- **Runtime Prerequisites:** Requires specific permissions depending on the underlying connection type (e.g., `BLUETOOTH_CONNECT`, `BLUETOOTH_ADVERTISE`, `UWB_RANGING`, `CHANGE_NETWORK_STATE`, or USB device access).
- **BOOT_COMPLETED Behavior:** Permitted to start from the background if the prerequisite permissions are granted and the device connection is valid.
- **Distribution Impact:** Standard Play Store distribution allowed if the app genuinely pairs with a connected device (which might be challenging to justify if we are just a TCP/WSS client connecting to a cloud server).

### Option B: `specialUse`
- **Purpose:** Designed as a catch-all for legitimate use cases that do not fit into any other defined FGS type.
- **Manifest Declaration:** `android:foregroundServiceType="specialUse"`
- **Android API Requirements:** API 34+.
- **Additional Declaration:** Must declare the `<property android:name="android.app.PROPERTY_SPECIAL_USE_FGS_SUBTYPE" android:value="..."/>` in the manifest to explain the exact use case.
- **Runtime Prerequisites:** None explicitly defined by the API, but requires the standard `FOREGROUND_SERVICE` and `FOREGROUND_SERVICE_SPECIAL_USE` permissions.
- **BOOT_COMPLETED Behavior:** Permitted to start from the background.
- **Play Distribution Impact:** Applications distributed via Google Play using `specialUse` are subject to strict manual review. If the use case is not deemed justifiable by Google reviewers, the app update may be rejected.
- **Private/Sideload Distribution Impact:** If distributed via sideloading (enterprise deployment), the strict Play Store review process does not apply, making it a highly viable option for internal/farm usage.

## 3. Decision

We will **VALIDATE IN PHASE 4 ON API 34/35**.

We will not declare `connectedDevice|specialUse` as a fallback combination, because declaring multiple types does not mean the OS will automatically choose the most favorable one; rather, the app must satisfy the prerequisites for the types it attempts to use.

During Phase 4, we will conduct rigorous physical testing on Android 14/15 devices:
1. Implement Option B (`specialUse`) first and verify `BOOT_COMPLETED` recovery via sideloaded APK.
2. Evaluate Option A (`connectedDevice`) if Option B presents unforeseen runtime limitations on certain OEM ROMs.
3. Finalize the single, verified FGS type in the Phase 4 Owner Gate.

## 4. Consequences
- **Positive:** We avoid premature commitment to an FGS type that might fail Play Store review or fail to start on certain strict OEM ROMs (like HyperOS or OneUI).
- **Negative:** Defers the final architectural declaration to Phase 4, requiring extra physical testing effort.
