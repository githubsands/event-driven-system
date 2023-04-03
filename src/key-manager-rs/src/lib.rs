use hyper::{Body, Request, Response};
use routerify::{Router};
use etcd_client::{Client, Error};
use tokio::sync::oneshot::{channel, Sender, Receiver};
use serde_json::to_string;

use serde_json::from_slice;
use serde::{Deserialize, Serialize};
use std::{convert::Infallible, net::SocketAddr};
use std::time::{SystemTime, UNIX_EPOCH};

use std::sync::{Arc, Mutex};
use deadqueue::unlimited::Queue;

type TaskQueue = deadqueue::limited::Queue<usize>;

/*
static ETCD_PRIVATE_KEY_STACK: RangeOptions= RangeOptions::new().with_prefix("private_key_stack/").with_sort_target();
static ETCD_QUEUE_RECOVERY: RangeOptions = RangeOptions::new().with_prefix("queue/").with_sort_target();
static ETCD_INUSE_KEYS: RangeOptions = RangeOptions::new().with_prefix("in_use_keys/").with_sort_target();
*/

static ETCD_CLUSTER_URI: &str = "etcd-0.demo.svc.cluster.local";
static HTTP_URI_PATH_STARTER: &str =  "/starter";
static HTTP_URI_PATH_IN_USE_KEYS_PATH: &str =  "/in_use_keys";
static HTTP_URI_PATH_PRIVATE_KEY_STACK: &str =  "/private_key_stack";

#[derive(Clone)]
pub struct PrivateKey {
    key: &'static str,
}


pub struct Transaction {
    sender: Arc<Sender<PrivateKey>>,
}

#[derive(Serialize, Deserialize, Debug)]
struct KeySignerRequest {
    current_private_key: String,
    worker_id: u32,
}

#[derive(Serialize, Deserialize, Debug)]
struct KeySignerResponse{
    updated_private_key: String,
}

impl Transaction {
    pub fn new(s: Arc<Sender<PrivateKey>>) -> Transaction {
        let (sender, receiver) = channel::<PrivateKey>();
        Self {
            sender: s,
        }
    }
    pub fn send(self, private_key: PrivateKey) -> Result<(), PrivateKey> {
        match Arc::try_unwrap(self.sender) {
            Ok(sender) =>  sender.send(private_key),
            Err(_) => Ok(())
        }
    }
}

impl Clone for Transaction {
    fn clone(&self) -> Self {
        Self {
            sender: Arc::clone(&self.sender),
        }
    }
}

pub struct KeyManager {
    queue: Arc<Queue<Transaction>>,
    // http_router: Router<Body, Infallible>,
    etcd_client: Client,
}

impl KeyManager {
    async fn new(&self) -> Result<KeyManager, Error> {
        let client = Client::connect([ETCD_CLUSTER_URI], None).await?;
        let router = self.new_router();
        let km = KeyManager {
            queue: Arc::new(Queue::new()),
            http_router: router,
            etcd_client: client,
        };
        Ok(km)
    }
    fn new_router(&self) -> Router<Body, Infallible> {
        let router = Router::builder()
            .get("/starter/:userId", self.handle_key_rotate_start)
            .get("/in_use_keys/:userId", self.handle_key_rotate)
            .get("/private_key_stack/:userId", self.handle_key_recover)
            .build()
            .unwrap();
        return router
    }
    /*
    async fn get_new_private_key(&mut self) -> Result<(), Error> {
        let resp = self.etcd_client.lock("/private_key_stack/", None).await?;
    }
    async fn etcd_get_private_keys_rotate(&mut self) -> Result<(), Error> {
        let resp = self.etcd_client.get(bob.key(), None).await?;
    }
    */
    /*
    async fn etcd_lock_private_key(&mut self) -> Result<(), Error> {
       let start = SystemTime::now();
       let resp = self.etcd_client.lock("/private_key_stack/", None).await?;
      // let resp = client.get(alice.key(), None).await?;

       // let key = resp.kvs().first() {
       // let Some(kv) = kv.value_str()?
    }
    */
    /*
    async fn etcd_initiate_worker_etcd(&self) -> Result<Response<Body>, Infallible> {

    }
    async fn assign_keys(&self) -> Result<Response<Body>, Infallible> {

    }
    */
    /*
    async fn etcd_recover_in_progress_key(&mut self, id: u8) -> Result<(), Error> {
        let resp = self.etcd_client.get(id, None).await?;
        Ok(())
    }
    */
    async fn handle_key_rotate_start(&self, req: Request<Body>) -> Result<Response<Body>, Infallible> {
        Ok(Response::new(Body::from(format!("Hello"))))
    }
    async fn handle_key_rotate(&mut self, req: Request<Body>) -> Result<Response<Body>, Infallible> {
        let body_bytes = hyper::body::to_bytes(req.into_body()).await.unwrap();
        let keySignerRequest: KeySignerRequest = from_slice(&body_bytes).unwrap();
        let (sender, receiver) = channel::<PrivateKey>();
        let transaction = Transaction::new(Arc::new(sender));
        self.queue.push(transaction.clone());
        match receiver.await {
            Ok(v) => {
                let ksr = KeySignerResponse{updated_private_key: "test".to_string()};
                let response_json = to_string(&ksr).unwrap();
                let response = Response::builder()
                .status(202)
                .header("Content-Type", "application/json")
                .body(Body::from(response_json))
                .unwrap();
                Ok(response)
            }
            Err(_) => {
                let response = Response::builder()
                .status(404)
                .header("Content-Type", "application/json")
                .body(Body::from("failed etcd key rotation"))
                .unwrap();
                Ok(response)
        }
    }
    }
}
    /*
    async fn handle_key_recover(&self, req: Request<Body>) -> Result<Response<Body>, Infallible> {
        Ok(Response::new(Body::from(
    }
    */
    /*
    async fn process_transaction(&self) {
         loop {
                let mut transaction = self.queue.pop().await;
                // let new_private_key = self.process_transaction_etcd(transaction.private_key()).await.unwrap();
                let new_private_key = "test";
                transaction.send(new_private_key.to_string());
            }
    }
    */
    /*
    async fn process_transaction_etcd(&self, private_key_in_use: String) -> Result<String, Error> {


    }
    */
    // async fn process_queue 

