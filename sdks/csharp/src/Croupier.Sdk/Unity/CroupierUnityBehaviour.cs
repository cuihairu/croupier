// Copyright 2025 Croupier Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

#if UNITY_5_3_OR_NEWER || UNITY_2017_1_OR_NEWER

using UnityEngine;
using Croupier.Sdk.Threading;

namespace Croupier.Sdk.Unity;

/// <summary>
/// Unity MonoBehaviour that automatically processes the main thread dispatcher queue.
///
/// Usage:
/// 1. Add this component to a GameObject in your scene
/// 2. The dispatcher will be automatically initialized and processed each frame
/// 3. Use MainThreadDispatcher.Instance.Enqueue() from any thread
///
/// Alternatively, create this component programmatically:
/// <code>
/// var go = new GameObject("CroupierDispatcher");
/// go.AddComponent&lt;CroupierUnityBehaviour&gt;();
/// DontDestroyOnLoad(go);
/// </code>
/// </summary>
[AddComponentMenu("Croupier/Croupier Unity Behaviour")]
[DisallowMultipleComponent]
public class CroupierUnityBehaviour : MonoBehaviour
{
    private static CroupierUnityBehaviour? _instance;

    [Tooltip("Maximum number of callbacks to process per frame. Set to 0 for unlimited.")]
    [SerializeField]
    private int maxCallbacksPerFrame = 100;

    [Tooltip("If true, this GameObject will persist across scene loads.")]
    [SerializeField]
    private bool dontDestroyOnLoad = true;

    /// <summary>
    /// Gets the singleton instance of the behaviour.
    /// </summary>
    public static CroupierUnityBehaviour? Instance => _instance;

    /// <summary>
    /// Gets or sets the maximum callbacks per frame.
    /// </summary>
    public int MaxCallbacksPerFrame
    {
        get => maxCallbacksPerFrame;
        set
        {
            maxCallbacksPerFrame = value;
            MainThreadDispatcher.Instance.SetMaxProcessPerFrame(
                value > 0 ? value : int.MaxValue
            );
        }
    }

    private void Awake()
    {
        // Ensure singleton
        if (_instance != null && _instance != this)
        {
            Debug.LogWarning("[Croupier] Multiple CroupierUnityBehaviour instances detected. Destroying duplicate.");
            Destroy(gameObject);
            return;
        }

        _instance = this;

        // Initialize the dispatcher on the main thread
        MainThreadDispatcher.Initialize();
        MainThreadDispatcher.Instance.SetMaxProcessPerFrame(
            maxCallbacksPerFrame > 0 ? maxCallbacksPerFrame : int.MaxValue
        );

        if (dontDestroyOnLoad)
        {
            DontDestroyOnLoad(gameObject);
        }

        Debug.Log("[Croupier] MainThreadDispatcher initialized");
    }

    private void Update()
    {
        // Process the dispatcher queue each frame
        MainThreadDispatcher.Instance.ProcessQueue();
    }

    private void OnDestroy()
    {
        if (_instance == this)
        {
            MainThreadDispatcher.Instance.Clear();
            _instance = null;
        }
    }

    /// <summary>
    /// Ensures the dispatcher is set up. Call this if you need to use the dispatcher
    /// before the Awake method runs.
    /// </summary>
    public static void EnsureInitialized()
    {
        if (_instance != null) return;

        var go = new GameObject("CroupierDispatcher");
        go.AddComponent<CroupierUnityBehaviour>();
    }
}

#endif
