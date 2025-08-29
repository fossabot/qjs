#include "qjs.h"

// toString method for QJS_PROXY_VALUE class
static JSValue qjs_proxy_value_toString(JSContext *ctx, JSValueConst this_val, int argc, JSValueConst *argv)
{
  JSValue proxy_id = JS_GetPropertyStr(ctx, this_val, "proxyId");
  if (JS_IsException(proxy_id))
    return proxy_id;

  const char *proxy_id_str = JS_ToCString(ctx, proxy_id);
  JS_FreeValue(ctx, proxy_id);

  if (!proxy_id_str)
    return JS_EXCEPTION;

  char buffer[256];
  snprintf(buffer, sizeof(buffer), "[object QJS_PROXY_VALUE(proxyId: %s)]", proxy_id_str);
  JS_FreeCString(ctx, proxy_id_str);

  return JS_NewString(ctx, buffer);
}

// Constructor function for QJS_PROXY_VALUE class
static JSValue qjs_proxy_value_constructor(JSContext *ctx, JSValueConst new_target, int argc, JSValueConst *argv)
{
  JSValue obj;
  JSValue proto;

  if (JS_IsUndefined(new_target)) {
    // Called as function, not constructor
    return JS_ThrowTypeError(ctx, "QJS_PROXY_VALUE must be called with new");
  }

  // Get prototype from new_target
  proto = JS_GetPropertyStr(ctx, new_target, "prototype");
  if (JS_IsException(proto))
    return proto;

  // Create object with proper prototype
  obj = JS_NewObjectProto(ctx, proto);
  JS_FreeValue(ctx, proto);

  if (JS_IsException(obj))
    return obj;

  // Set the proxyId property
  if (argc > 0) {
    if (JS_SetPropertyStr(ctx, obj, "proxyId", JS_DupValue(ctx, argv[0])) < 0) {
      JS_FreeValue(ctx, obj);
      return JS_EXCEPTION;
    }
  } else {
    if (JS_SetPropertyStr(ctx, obj, "proxyId", JS_UNDEFINED) < 0) {
      JS_FreeValue(ctx, obj);
      return JS_EXCEPTION;
    }
  }

  return obj;
}

// Initialize QJS_PROXY_VALUE class and add it to global object
static int init_qjs_proxy_value_class(JSContext *ctx)
{
  JSValue global_obj = JS_GetGlobalObject(ctx);

  // Create prototype object with toString method
  JSValue proto = JS_NewObject(ctx);
  JSValue toString_func = JS_NewCFunction(ctx, qjs_proxy_value_toString, "toString", 0);
  if (JS_SetPropertyStr(ctx, proto, "toString", toString_func) < 0) {
    JS_FreeValue(ctx, proto);
    JS_FreeValue(ctx, global_obj);
    return -1;
  }

  // Create the constructor function
  JSValue ctor = JS_NewCFunction2(ctx, qjs_proxy_value_constructor, "QJS_PROXY_VALUE", 1, JS_CFUNC_constructor, 0);

  // Set proto.constructor and ctor.prototype using QuickJS helper
  JS_SetConstructor(ctx, ctor, proto);

  // Add the constructor to the global object
  if (JS_SetPropertyStr(ctx, global_obj, "QJS_PROXY_VALUE", ctor) < 0) {
    JS_FreeValue(ctx, global_obj);
    return -1;
  }

  JS_FreeValue(ctx, global_obj);
  return 0;
}

JSContext *New_QJSContext(JSRuntime *rt)
{
  JSContext *ctx;
  ctx = JS_NewContext(rt);
  js_init_module_std(ctx, "qjs:std");
  js_init_module_os(ctx, "qjs:os");
  js_init_module_bjson(ctx, "qjs:bjson");
  js_set_global_objs(ctx);

  return ctx;
}

QJSRuntime *New_QJS(
    size_t memory_limit,
    size_t max_stack_size,
    size_t max_execution_time,
    size_t gc_threshold)
{
  JSRuntime *runtime;
  JSContext *ctx;

  runtime = JS_NewRuntime();

  if (!runtime)
    return NULL;

  if (memory_limit > 0)
    JS_SetMemoryLimit(runtime, memory_limit);

  if (gc_threshold > 0)
    JS_SetGCThreshold(runtime, gc_threshold);

  if (max_stack_size > 0)
    JS_SetMaxStackSize(runtime, max_stack_size);

  /* setup the the worker context */
  js_std_set_worker_new_context_func(New_QJSContext);
  /* initialize the standard objects */
  js_std_init_handlers(runtime);
  /* loader for ES6 modules */
  JS_SetModuleLoaderFunc(runtime, NULL, QJS_ModuleLoader, NULL);
  /* exit on unhandled promise rejections */
  // JS_SetHostPromiseRejectionTracker(runtime, js_std_promise_rejection_tracker, NULL);

  ctx = New_QJSContext(runtime);
  if (!ctx)
  {
    JS_FreeRuntime(runtime);
    return NULL;
  }

  // Initialize QJS_PROXY_VALUE class
  if (init_qjs_proxy_value_class(ctx) < 0) {
    JS_FreeContext(ctx);
    JS_FreeRuntime(runtime);
    return NULL;
  }

  QJSRuntime *qjs = (QJSRuntime *)malloc(sizeof(QJSRuntime));
  if (!qjs) {
    JS_FreeContext(ctx);
    JS_FreeRuntime(runtime);
    return NULL;
  }

  qjs->runtime = runtime;
  qjs->context = ctx;

  return qjs;
}

void QJS_FreeValue(JSContext *ctx, JSValue val)
{
  JS_FreeValue(ctx, val);
}

void QJS_Free(QJSRuntime *qjs)
{
  JS_FreeContext(qjs->context);
  JS_FreeRuntime(qjs->runtime);
  free(qjs);
}

JSValue QJS_CloneValue(JSContext *ctx, JSValue val)
{
  return JS_DupValue(ctx, val);
}

JSContext *QJS_GetContext(QJSRuntime *qjs)
{
  return qjs->context;
}

void QJS_UpdateStackTop(QJSRuntime *qjs)
{
  JS_UpdateStackTop(qjs->runtime);
}

QJSRuntime *qjs = NULL;

QJSRuntime *QJS_GetRuntime()
{
  return qjs;
}

void initialize()
{
  if (qjs != NULL)
    return;
  size_t memory_limit = 0;
  size_t gc_threshold = 0;
  size_t max_stack_size = 0;
  qjs = New_QJS(memory_limit, max_stack_size, 0, gc_threshold);
}
