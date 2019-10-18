import React, { useState } from "react";
import $ from "jquery";
import auth0 from "auth0-js";
// import "./index.css";
import axios from "axios";
import "antd/dist/antd.css";
import "./online-litfest.css";
import { Editor } from "react-draft-wysiwyg";
import "react-draft-wysiwyg/dist/react-draft-wysiwyg.css";
import createCounterPlugin from "draft-js-counter-plugin";
import {
  Upload,
  message,
  Button,
  Icon,
  Spin,
  Divider,
  Modal as Confirm,
  Typography
} from "antd";
import { ModalHeader, ModalBody, Modal } from "reactstrap";
import draftToHtml from "draftjs-to-html";
import { fadeIn } from "react-animations";

import Logo from "./assets/zozimus.png";
import event from "./events";

const { confirm } = Confirm;

const { Paragraph } = Typography;

let globalRootURL = "https://" + window.location.host;

const AUTH0_CLIENT_ID = "JCvzA0JNJ5KCNONawklcvWx2MT1s53eK";
const AUTH0_DOMAIN = "online-litfest.auth0.com";
const AUTH0_API_AUDIENCE = "https://online-litfest.auth0.com/api/v2/";
let AUTH0_CALLBACK_URL = globalRootURL;

const loading = <Icon type="loading" style={{ fontSize: 48 }} spin />;

const { Dragger } = Upload;

const content = {
  entityMap: {},
  blocks: [
    {
      key: "637gr",
      text: "Initialized from content state.",
      type: "unstyled",
      depth: 0,
      inlineStyleRanges: [],
      entityRanges: [],
      data: {}
    }
  ]
};
const ModalExample = props => {
  const {
    buttonLabel,
    className,
    register,
    eventName,
    eventID,
    wordLimit,
    entryLimit
  } = props;

  const [modal, setModal] = useState(false);

  const toggle = () => setModal(!modal);

  return (
    <div>
      <Button
        color="primary"
        onClick={buttonLabel === "Participate" ? toggle : register}
      >
        {buttonLabel}
      </Button>
      <Modal
        isOpen={modal}
        toggle={toggle}
        className={className}
        zIndex="50"
        size="lg"
        style={{ height: "500px" }}
      >
        <ModalHeader toggle={toggle}>
          New Submission | {eventName}, Zozimus 2019
        </ModalHeader>
        <ModalBody style={{ textAlign: "center" }}>
          <SubmissionControl
            EventName={eventName}
            eventID={eventID}
            wordLimit={wordLimit}
            entryLimit={entryLimit}
          />
        </ModalBody>
      </Modal>
    </div>
  );
};

class App extends React.Component {
  state = {
    visible: false,
    loggedIn: null,
    event: window.location.pathname.split("/")[2]
  };
  constructor() {
    super();
    this.setup();
    this.parseHash();
    this.authenticate = this.authenticate.bind(this);
  }
  showModal = () => {
    this.setState({ visible: true });
  };
  handleOk = e => {
    this.setState({ visible: false });
  };
  authenticate() {
    this.WebAuth = new auth0.WebAuth({
      domain: AUTH0_DOMAIN,
      clientID: AUTH0_CLIENT_ID,
      scope: "openid email",
      audience: AUTH0_API_AUDIENCE,
      responseType: "token id_token",
      redirectUri: AUTH0_CALLBACK_URL
    });
    this.WebAuth.authorize();
  }
  parseHash() {
    this.auth0 = new auth0.WebAuth({
      domain: AUTH0_DOMAIN,
      clientID: AUTH0_CLIENT_ID
    });
    this.auth0.parseHash((err, authResult) => {
      if (err) {
        return console.log(err);
      }
      if (
        authResult !== null &&
        authResult.accessToken !== null &&
        authResult.idToken !== null
      ) {
        localStorage.setItem("access_token", authResult.accessToken);
        localStorage.setItem("id_token", authResult.idToken);
        localStorage.setItem(
          "email",
          JSON.stringify(authResult.idTokenPayload)
        );
        window.location = window.location.href.substr(
          0,
          window.location.href.indexOf("")
        );
      }
    });
  }
  setup() {
    $.ajaxSetup({
      beforeSend: function(xhr) {
        if (localStorage.getItem("access_token")) {
          xhr.setRequestHeader(
            "Authorization",
            "Bearer " + localStorage.getItem("access_token")
          );
        }
      }
    });
  }
  setState() {
    let idToken = localStorage.getItem("id_token");
    if (idToken) {
      this.state.loggedIn = true;
    } else {
      this.state.loggedIn = false;
    }
  }
  componentDidMount() {
    AUTH0_CALLBACK_URL =
      globalRootURL + "/portal/" + window.location.pathname.split("/")[2];
    console.log(AUTH0_CALLBACK_URL);
    console.log(window.location.pathname.split("/")[2]);
  }

  render() {
    if (
      this.state.event == "sweetheart" ||
      this.state.event == "memeify" ||
      this.state.event == "essay" ||
      this.state.event == "goosebumps" ||
      this.state.event == "dearme" ||
      this.state.event == "ragtag" ||
      this.state.event == "review" ||
      this.state.event == "plot" ||
      this.state.event == "poetry"
    ) {
      let rulesObject = [];
      event[this.state.event].rules.forEach(item => {
        rulesObject.push(<li dangerouslySetInnerHTML={{ __html: item }}></li>);
      });
      return (
        <div
          style={{
            backgroundColor: event[this.state.event].backgroundColor,
            fontSize: "1.3rem"
          }}
        >
          <nav className="fh5co-nav" role="navigation" id="navbar">
            <div className="container-fluid">
              <div className="row">
                <div className="col-md-3">
                  <div id="fh5co-logo">
                    <a href="https://elan.org.in/litfest.html">
                      <img src={Logo} />
                    </a>
                  </div>
                </div>
                <div className="col-md-9 text-right menu-1">
                  <ul style={{ marginRight: "3rem" }}>
                    <li className="active">
                      <a href="https://elan.org.in/litfest.html">Home</a>
                    </li>
                    <li>
                      <a href="https://elan.org.in/litfest-rules.html">Rules</a>
                    </li>
                    <li>
                      <a href="https://elan.org.in/litfest-contact.html">
                        Contact Us
                      </a>
                    </li>
                  </ul>
                </div>
              </div>
            </div>
          </nav>
          <div id="page">
            <div id="fh5co-first animated fadeIn">
              <div className="container-fluid">
                <div className="row no-gutters">
                  <div className="col-lg-5 col-xl-6">
                    <div className="event-about">
                      <div className="fh5co-heading animate-box">
                        <h1>{event[this.state.event].name}</h1>
                      </div>
                      <div className="animate-box">
                        {this.state.event == "goosebumps" ? (
                          <blockquote className="blockquote">
                            <p className="mb-0">
                              {event[this.state.event].blockquote}
                            </p>
                            <footer className="blockquote-footer">
                              <cite title="Source Title">
                                {event[this.state.event].blockquoteAuthor}
                              </cite>
                            </footer>
                          </blockquote>
                        ) : (
                          <p></p>
                        )}
                        <div
                          dangerouslySetInnerHTML={{
                            __html: event[this.state.event].description
                          }}
                        ></div>
                        <br />

                        <div className="text-center">
                          <ModalExample
                            buttonLabel={
                              localStorage.getItem("id_token")
                                ? "Participate"
                                : "Register"
                            }
                            register={this.authenticate}
                            eventName={event[this.state.event].name}
                            eventID={event[this.state.event].id}
                            wordLimit={event[this.state.event].wordLimit}
                            entryLimit={event[this.state.event].entryLimit}
                          />
                        </div>
                        <ul className="text-center">
                          <li>
                            <a href="#rules">Rules</a>
                          </li>
                          {/* <li>
                            <a href="litfest-rules.html">Sample-Entry</a>
                          </li> */}
                          <li>
                            <a href="#prizes">Prizes</a>
                          </li>
                        </ul>
                      </div>
                    </div>
                  </div>
                  <div
                    className="col-lg-7 col-xl-6 event-poster animate-box text-right"
                    data-animate-effect="fadeIn"
                  >
                    <img src={event[this.state.event].imageLink} />
                  </div>
                </div>
              </div>
            </div>
            <div id="prizes">
              <div id="fh5co-couple-story">
                <div className="container rules">
                  <div className="row">
                    <div className="col-md-12 justify-content-md-center text-center fh5co-heading animate-box">
                      <h1>The Prizes</h1>
                    </div>
                  </div>

                  <div className="row no-gutters text-center animate-box prizes">
                    <div className="col-md-6">
                      <h2>1st Prize</h2>
                      <h1>Rs {event[this.state.event].prize[0]}</h1>
                    </div>
                    <div className="col-md-6 second-prize">
                      <h2>2nd Prize</h2>
                      <h1>Rs {event[this.state.event].prize[1]}</h1>
                    </div>
                  </div>
                </div>
              </div>
            </div>

            <div id="rules">
              <div id="fh5co-couple-story" className="rules">
                <div className="container">
                  <div className="row">
                    <div className="col-md-12 justify-content-md-center text-center fh5co-heading animate-box">
                      <h1>The Rules</h1>
                    </div>
                  </div>
                  <div className="row animate-box">
                    <div className="col-12">
                      <ol className="" style={{ marginTop: "2vw" }}>
                        {rulesObject}
                      </ol>
                    </div>
                  </div>
                </div>
              </div>
            </div>
          </div>

          <footer id="fh5co-footer" role="contentinfo">
            <div className="container">
              <div className="row copyright">
                <div className="col-md-12 text-center">
                  <p>
                    <small className="block">
                      Website developed and maintained by Lambda, IIT Hyderabad
                    </small>
                  </p>
                  <ul className="fh5co-social-icons">
                    <li>
                      <a href="https://www.facebook.com/elan.iithyderabad/">
                        <i className="icon-facebook"></i>
                      </a>
                    </li>
                    <li>
                      <a href="https://www.instagram.com/elan_nvision.iith/?hl=en">
                        <i className="icon-instagram"></i>
                      </a>
                    </li>
                    <li>
                      <a href="https://twitter.com/elan_nvision?lang=en">
                        <i className="icon-twitter"></i>
                      </a>
                    </li>
                  </ul>
                </div>
              </div>
            </div>
          </footer>

          <div className="gototop js-top">
            <a href="#" className="js-gotop">
              <i className="icon-arrow-up"></i>
            </a>
          </div>
        </div>
      );
    } else {
      return <p>404 not found</p>;
    }
  }
}

class SubmissionControl extends React.Component {
  constructor(props) {
    super(props);
  }
  render() {
    switch (this.props.EventName) {
      case "Memeify":
        return <Memeify />;
        break;
      default:
        return (
          <EditorThing
            entryLimit={this.props.entryLimit}
            wordLimit={this.props.wordLimit}
            eventName={this.props.eventID}
          />
        );
    }
  }
}

class Memeify extends React.Component {
  constructor() {
    super();
    this.state = { uploads: null };
    this.beforeUpload = this.beforeUpload.bind(this);
  }
  beforeUpload(file) {
    const isJpgOrPng =
      file.type === "image/jpeg" ||
      file.type === "image/png" ||
      file.type === "application/pdf" ||
      file.type === "image/gif";
    if (!isJpgOrPng) {
      message.error("Please upload only JPEG/PNG/PDF/GIF images. ");
    }
    const isLt2M = file.size / 1024 / 1024 < 2;
    if (!isLt2M) {
      message.error("Your file is too large. :/");
    }

    if (isJpgOrPng && isLt2M) {
      this.setState({ uploads: this.state.uploads + 1 });
    }

    return isJpgOrPng && isLt2M;
  }
  componentDidMount() {
    const headers = {
      headers: {
        "Content-Type": "application/json",
        authorization: "Bearer " + localStorage.getItem("access_token")
      }
    };
    let url =
      globalRootURL +
      "/api/private/memeify/entries?id_token=" +
      localStorage.getItem("id_token");
    axios
      .get(url, headers)
      .then(res => {
        this.setState({ uploads: res.data.entries });
      })
      .catch(error => {
        localStorage.clear();
        window.location.reload();
      });
  }
  render() {
    const props = {
      name: "file",
      action:
        globalRootURL +
        "/api/private/memeify/upload?id_token=" +
        localStorage.getItem("id_token"),
      headers: {
        authorization: "Bearer " + localStorage.getItem("access_token")
      },
      onChange(info) {
        if (info.file.status !== "uploading") {
          console.log(info.file, info.fileList);
        }
        if (info.file.status === "done") {
          message.success(`${info.file.name} file uploaded successfully`);
        } else if (info.file.status === "error") {
          message.error(`${info.file.name} file upload failed.`);
        }
      },
      beforeUpload: this.beforeUpload
    };
    if (this.state.uploads == null) {
      return <Spin size="large" indicator={loading}></Spin>;
    } else if (this.state.uploads > 3) {
      return <p>You've already submitted 3 entries. </p>;
    } else if (this.state.uploads <= 3) {
      return (
        <Dragger {...props}>
          <p className="ant-upload-drag-icon">
            <Icon type="upload" />
          </p>
          <p className="ant-upload-text">
            Upload your meme as a jpg or png or pdf here.
          </p>
          <p className="ant-upload-hint">
            File size should be less than 2 MB. <br /> You can only upload a
            total of 3 memes. <br /> Once uploaded, memes CANNOT be deleted.
          </p>
        </Dragger>
      );
    }
  }
}

const counterPlugin = createCounterPlugin();
const { CharCounter, WordCounter, LineCounter, CustomCounter } = counterPlugin;
const plugins = [counterPlugin];

class EditorThing extends React.Component {
  constructor(props) {
    super(props);
    const contentState = content;
    this.state = {
      contentState,
      uploads: null
    };
    this.onContentStateChange = this.onContentStateChange.bind(this);
    this.submitHTML = this.submitHTML.bind(this);
    this.beforeUpload = this.beforeUpload.bind(this);
  }
  onContentStateChange(contentState) {
    this.setState({
      contentState
    });
  }
  countWords(html) {
    var tmp = document.createElement("DIV");
    tmp.innerHTML = html;
    return (tmp.textContent || tmp.innerText || "").split(" ").length;
  }
  submitHTML() {
    let word_limit = this.props.wordLimit;
    if (
      this.state.uploads < this.props.entryLimit &&
      this.countWords(draftToHtml(this.state.contentState)) <= word_limit
    ) {
      console.log(draftToHtml(this.state.contentState));
      const headers = {
        headers: {
          "Content-Type": "application/json",
          authorization: "Bearer " + localStorage.getItem("access_token")
        }
      };
      let url =
        globalRootURL +
        "/api/private/" +
        this.props.eventName +
        "/upload?id_token=" +
        localStorage.getItem("id_token");
      axios
        .post(url, draftToHtml(this.state.contentState), headers)
        .catch(error => {
          localStorage.clear();
          window.location.reload();
        });
      message.success("We have received your submission. Redirecting you...");
      setTimeout(function() {
        window.location.reload();
      }, 3000);
    } else if (this.state.uploads >= this.props.entryLimit) {
      message.error(
        "You've already submitted " +
          this.props.entryLimit +
          " entries. That's the maximum number allowed for this event. "
      );
    } else {
      message.error(
        "Too many words! Adhere to the word limit: " +
          this.props.wordLimit +
          ". "
      );
    }
  }
  componentDidMount() {
    const headers = {
      headers: {
        "Content-Type": "application/json",
        authorization: "Bearer " + localStorage.getItem("access_token")
      }
    };
    let url =
      globalRootURL +
      "/api/private/" +
      this.props.eventName +
      "/entries?id_token=" +
      localStorage.getItem("id_token");
    axios
      .get(url, headers)
      .then(res => {
        this.setState({ uploads: res.data.entries });
      })
      .catch(error => {
        localStorage.clear();
        window.location.reload();
      });
  }
  beforeUpload(file) {
    const isJpgOrPng = file.type === "application/pdf";
    if (!isJpgOrPng) {
      message.error("Please upload only PDF documents. ");
    }
    const isLt2M = file.size / 1024 / 1024 < 2;
    if (!isLt2M) {
      message.error(
        "Your file is too large. Make sure it's less than 2 MB. :/"
      );
    }

    if (isJpgOrPng && isLt2M) {
      this.setState({ uploads: this.state.uploads + 1 });
    }

    return isJpgOrPng && isLt2M;
  }
  render() {
    const props = {
      name: "file",
      action:
        globalRootURL +
        "/api/private/" +
        this.props.eventName +
        "/pdf?id_token=" +
        localStorage.getItem("id_token"),
      headers: {
        authorization: "Bearer " + localStorage.getItem("access_token")
      },
      onChange(info) {
        if (info.file.status !== "uploading") {
          console.log(info.file, info.fileList);
        }
        if (info.file.status === "done") {
          message.success(`${info.file.name} file uploaded successfully`);
        } else if (info.file.status === "error") {
          message.error(`${info.file.name} file upload failed.`);
        }
      },
      beforeUpload: this.beforeUpload
    };
    const { contentState } = this.state;
    if (this.state.uploads == null) {
      return <Spin size="large" indicator={loading}></Spin>;
    } else if (this.state.uploads > this.props.entryLimit) {
      return <p>You've already submitted {this.props.entryLimit} entries. </p>;
    } else if (this.state.uploads <= this.props.entryLimit) {
      return (
        <div>
          <Editor
            wrapperClassName="demo-wrapper"
            editorClassName="demo-editor"
            onContentStateChange={this.onContentStateChange}
            editorStyle={{
              height: "60vh",
              overflow: "hidden",
              padding: "10px"
            }}
          />
          <Button
            type="primary"
            onClick={() => {
              confirm({
                title: "Are you sure you wanna submit?",
                content:
                  "You will not be able to delete this entry and you have a total of " +
                  (this.props.entryLimit - this.state.uploads) +
                  " entries left. ",
                onOk: () => {
                  this.submitHTML();
                },
                onCancel() {
                  console.log("Cancel");
                }
              });
            }}
          >
            Submit
          </Button>
          <Divider>OR</Divider>
          <Upload {...props}>
            <Button type="primary" icon="file-pdf">
              Upload a PDF document
            </Button>
            <Paragraph style={{ paddingTop: "10px" }}>
              You have {this.props.entryLimit - this.state.uploads} entries
              left. Note that after clicking the upload button, you will not be
              able to delete your entry.{" "}
            </Paragraph>
          </Upload>
        </div>
      );
    }
  }
}

export default App;
