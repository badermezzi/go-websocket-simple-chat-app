let pc;
let localStream;
const configuration = { iceServers: [{ urls: 'stun:stun.l.google.com:19302' }] }; // Example STUN server

// --- Common Setup ---
async function setupMediaAndPC() {
  localStream = await navigator.mediaDevices.getUserMedia({ video: true, audio: true });
  // TODO: Display localStream in a <video> element

  pc = new RTCPeerConnection(configuration);

  // Send generated ICE candidates to the other peer via WebSocket
  pc.onicecandidate = (event) => {
    if (event.candidate) {
      sendIceCandidate(targetUserId, event.candidate); // Need targetUserId
    }
  };

  // Add local tracks so they are sent to the peer
  localStream.getTracks().forEach(track => pc.addTrack(track, localStream));

  // *** Crucial: Handle receiving the remote stream ***
  pc.ontrack = (event) => {
    console.log("Remote track received!");
    // TODO: Get event.streams[0] and display it in another <video> element
    // const remoteVideoElement = document.getElementById('remoteVideo');
    // if (remoteVideoElement) {
    //   remoteVideoElement.srcObject = event.streams[0];
    // }
  };
}

// --- Initiator Logic ---
async function startCall() {
  await setupMediaAndPC();
  const offer = await pc.createOffer();
  await pc.setLocalDescription(offer);
  sendOffer(targetUserId, offer); // Need targetUserId
}

// --- Receiver Logic (triggered by receiving offer via WebSocket) ---
async function handleOffer(offerSdp) {
  await setupMediaAndPC();
  await pc.setRemoteDescription(new RTCSessionDescription({ type: 'offer', sdp: offerSdp }));
  const answer = await pc.createAnswer();
  await pc.setLocalDescription(answer);
  sendAnswer(targetUserId, answer); // Need targetUserId
}

// --- Handling Answer (triggered on Initiator side) ---
async function handleAnswer(answerSdp) {
  if (pc && pc.signalingState === 'have-local-offer') { // Check state
     await pc.setRemoteDescription(new RTCSessionDescription({ type: 'answer', sdp: answerSdp }));
     console.log("Set remote answer successfully!");
  }
}

// --- Handling ICE Candidates (triggered on both sides) ---
async function handleIceCandidate(candidateData) {
  if (pc && candidateData) {
    try {
      await pc.addIceCandidate(new RTCIceCandidate(candidateData));
    } catch (e) {
      console.error('Error adding received ice candidate', e);
    }
  }
}

// WebSocket onmessage needs to route messages to these handlers:
// socket.onmessage = (event) => {
//   const message = JSON.parse(event.data);
//   if (message.type === 'offer') handleOffer(message.sdp);
//   else if (message.type === 'answer') handleAnswer(message.sdp);
//   else if (message.type === 'ice-candidate') handleIceCandidate(message.candidate);
//   // ...
// }